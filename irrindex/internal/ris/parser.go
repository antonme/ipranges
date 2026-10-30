package ris

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Record struct {
	ASN       string
	Prefix    string
	PeerCount int
	IsDefault bool
}

func Parse(r io.Reader) ([]Record, error) {
	scanner := bufio.NewScanner(r)

	var records []Record
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "%") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, fmt.Errorf("line %d: expected 3 fields, got %d", lineNo, len(fields))
		}

		peerCount, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("line %d: parse peer count: %w", lineNo, err)
		}

		prefix := fields[1]
		records = append(records, Record{
			ASN:       normalizeASN(fields[0]),
			Prefix:    prefix,
			PeerCount: peerCount,
			IsDefault: prefix == "0.0.0.0/0" || prefix == "::/0",
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return records, nil
}

func normalizeASN(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && strings.EqualFold(value[:2], "AS") {
		return "AS" + value[2:]
	}
	return "AS" + value
}
