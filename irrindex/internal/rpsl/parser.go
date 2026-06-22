package rpsl

import (
	"bufio"
	"io"
	"strings"
	"unicode/utf8"
)

type Object struct {
	Type    string
	Prefix  string
	StartIP string
	EndIP   string
	Attrs   map[string][]string
}

func Parse(r io.Reader) ([]Object, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var objects []Object
	var block []string
	for scanner.Scan() {
		line := safeString(scanner.Bytes())
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			objects = flushBlock(objects, block)
			block = nil
			continue
		}
		block = append(block, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	objects = flushBlock(objects, block)

	return objects, nil
}

func flushBlock(objects []Object, block []string) []Object {
	obj, ok := parseBlock(block)
	if !ok {
		return objects
	}
	return append(objects, obj)
}

func parseBlock(block []string) (Object, bool) {
	attrs := map[string][]string{}
	var lastKey string

	for _, raw := range block {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		if strings.HasPrefix(raw, "%") || strings.HasPrefix(raw, "#") {
			continue
		}
		if isContinuation(raw) {
			if lastKey == "" {
				continue
			}
			values := attrs[lastKey]
			if len(values) == 0 {
				continue
			}
			values[len(values)-1] += " " + strings.TrimSpace(raw)
			attrs[lastKey] = values
			continue
		}

		key, value, ok := strings.Cut(raw, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		attrs[key] = append(attrs[key], value)
		lastKey = key
	}

	if len(attrs) == 0 {
		return Object{}, false
	}

	obj := Object{Attrs: attrs}
	for _, key := range []string{"route", "route6", "inetnum", "inet6num"} {
		values := attrs[key]
		if len(values) == 0 {
			continue
		}
		obj.Type = key
		switch key {
		case "route", "route6", "inet6num":
			obj.Prefix = values[0]
		case "inetnum":
			obj.StartIP, obj.EndIP = parseInetnumRange(values[0])
			if obj.StartIP == "" {
				obj.Prefix = values[0]
			}
		}
		break
	}

	if obj.Type == "" {
		return Object{}, false
	}

	return obj, true
}

func parseInetnumRange(value string) (string, string) {
	start, end, ok := strings.Cut(value, "-")
	if !ok {
		return "", ""
	}
	start = strings.TrimSpace(start)
	end = strings.TrimSpace(end)
	if start == "" || end == "" {
		return "", ""
	}
	return start, end
}

func isContinuation(line string) bool {
	if line == "" {
		return false
	}
	return line[0] == ' ' || line[0] == '\t'
}

func safeString(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	return strings.ToValidUTF8(string(b), "\uFFFD")
}
