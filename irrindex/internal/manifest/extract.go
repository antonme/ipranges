package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/antonme/ipranges/irrindex/internal/config"
	"gopkg.in/yaml.v3"
)

type QuerySpec struct {
	Downloader string
	Kind       string
	Value      string
	Hosts      []string
	Mirrors    []string
}

type helperSpec struct {
	kind    string
	hosts   []string
	mirrors []string
}

var (
	helperStartRE = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*\(\)\s*\{\s*$`)
	whoisHostRE   = regexp.MustCompile(`whois\s+-h\s+([^\s]+)`)
	forLoopRE     = regexp.MustCompile(`(?ms)for\s+([A-Za-z_][A-Za-z0-9_]*)\s+in\s+([^;]+);\s*do(.*?)done`)
)

func ExtractQueries(repoRoot string) ([]QuerySpec, error) {
	entries, err := os.ReadDir(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("read repo root: %w", err)
	}

	var specs []QuerySpec
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		downloader := entry.Name()
		path := filepath.Join(repoRoot, downloader, "downloader.sh")
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}

		extracted := extractDownloaderQueries(downloader, string(data))
		specs = append(specs, extracted...)
	}

	sortQuerySpecs(specs)
	return specs, nil
}

func WriteQueriesYAML(path string, specs []QuerySpec) error {
	raw := struct {
		Queries []config.Query `yaml:"queries"`
	}{
		Queries: make([]config.Query, 0, len(specs)),
	}
	for _, spec := range specs {
		raw.Queries = append(raw.Queries, config.Query{
			Downloader: spec.Downloader,
			Kind:       spec.Kind,
			Value:      spec.Value,
			Mirrors:    append([]string(nil), spec.Mirrors...),
		})
	}

	data, err := yaml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal queries YAML: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create query manifest dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write query manifest: %w", err)
	}
	return nil
}

func extractDownloaderQueries(downloader string, script string) []QuerySpec {
	helpers := extractHelpers(script)
	seen := map[string]QuerySpec{}

	add := func(helperName string, value string) {
		helper, ok := helpers[helperName]
		if !ok {
			return
		}
		value = strings.Trim(value, `"'`)
		if value == "" || strings.HasPrefix(value, "$") {
			return
		}
		if helper.kind == "origin" && !isOriginASN(value) {
			return
		}
		spec := QuerySpec{
			Downloader: downloader,
			Kind:       helper.kind,
			Value:      value,
			Hosts:      append([]string(nil), helper.hosts...),
			Mirrors:    append([]string(nil), helper.mirrors...),
		}
		seen[specKey(spec)] = spec
	}

	for _, line := range strings.Split(script, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		add(fields[0], fields[1])
	}

	for _, match := range forLoopRE.FindAllStringSubmatch(script, -1) {
		varName := match[1]
		values := strings.Fields(match[2])
		body := match[3]
		for helperName := range helpers {
			loopCallRE := regexp.MustCompile(regexp.QuoteMeta(helperName) + `\s+["']?\$` + regexp.QuoteMeta(varName) + `["']?`)
			if !loopCallRE.MatchString(body) {
				continue
			}
			for _, value := range values {
				add(helperName, value)
			}
		}
	}

	specs := make([]QuerySpec, 0, len(seen))
	for _, spec := range seen {
		specs = append(specs, spec)
	}
	sortQuerySpecs(specs)
	return specs
}

func isOriginASN(value string) bool {
	if len(value) <= 2 || !strings.HasPrefix(strings.ToUpper(value), "AS") {
		return false
	}
	for _, ch := range value[2:] {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func extractHelpers(script string) map[string]helperSpec {
	helpers := map[string]helperSpec{}

	lines := strings.Split(script, "\n")
	for i := 0; i < len(lines); i++ {
		match := helperStartRE.FindStringSubmatch(lines[i])
		if match == nil {
			continue
		}
		name := match[1]
		var body []string
		i++
		for ; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "}" {
				break
			}
			body = append(body, lines[i])
		}

		if helper, ok := parseHelperBody(body); ok {
			helpers[name] = helper
		}
	}
	return helpers
}

func parseHelperBody(lines []string) (helperSpec, bool) {
	var kind string
	var hosts []string
	var mirrors []string
	for _, line := range lines {
		hostMatch := whoisHostRE.FindStringSubmatch(line)
		if hostMatch == nil {
			continue
		}
		lineKind := ""
		switch {
		case strings.Contains(line, "-i origin $1"):
			lineKind = "origin"
		case strings.Contains(line, "-i mnt-by $1"):
			lineKind = "mnt-by"
		default:
			continue
		}
		if kind == "" {
			kind = lineKind
		}
		if lineKind != kind {
			continue
		}
		host := hostMatch[1]
		hosts = appendUnique(hosts, host)
		mirrors = appendUnique(mirrors, mirrorName(host))
	}
	if kind == "" || len(hosts) == 0 {
		return helperSpec{}, false
	}
	return helperSpec{kind: kind, hosts: hosts, mirrors: mirrors}, true
}

func mirrorName(host string) string {
	switch strings.ToLower(host) {
	case "riswhois.ripe.net":
		return "ris"
	case "whois.radb.net":
		return "radb"
	case "whois.ripe.net":
		return "ripe"
	case "rr.ntt.net":
		return "ntt"
	case "whois.rogerstelecom.net":
		return "rogers"
	case "whois.bgp.net.br":
		return "bgp_net_br"
	default:
		return host
	}
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func specKey(spec QuerySpec) string {
	return spec.Downloader + "\x00" + spec.Kind + "\x00" + spec.Value
}

func sortQuerySpecs(specs []QuerySpec) {
	sort.Slice(specs, func(i, j int) bool {
		if specs[i].Downloader != specs[j].Downloader {
			return specs[i].Downloader < specs[j].Downloader
		}
		if specs[i].Kind != specs[j].Kind {
			return specs[i].Kind < specs[j].Kind
		}
		return specs[i].Value < specs[j].Value
	})
}
