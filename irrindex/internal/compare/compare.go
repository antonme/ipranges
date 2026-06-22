package compare

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/antonme/ipranges/irrindex/internal/config"
	routequery "github.com/antonme/ipranges/irrindex/internal/query"
	"github.com/antonme/ipranges/irrindex/internal/store"
)

type Report struct {
	Services []ServiceReport `json:"services"`
}

type ServiceReport struct {
	Downloader       string   `json:"downloader"`
	Status           string   `json:"status"`
	LiveCount        int      `json:"live_count"`
	LocalCount       int      `json:"local_count"`
	InvalidLiveCount int      `json:"invalid_live_count"`
	LocalOnlyCount   int      `json:"local_only_count"`
	LiveOnlyCount    int      `json:"live_only_count"`
	InvalidLive      []string `json:"invalid_live,omitempty"`
	LocalOnly        []string `json:"local_only,omitempty"`
	LiveOnly         []string `json:"live_only,omitempty"`
}

func BuildReport(repoRoot string, queries []config.Query, db *store.DB) (Report, error) {
	byDownloader := map[string][]config.Query{}
	for _, q := range queries {
		byDownloader[q.Downloader] = append(byDownloader[q.Downloader], q)
	}

	report := Report{Services: make([]ServiceReport, 0, len(byDownloader))}
	for downloader, downloaderQueries := range byDownloader {
		live, invalidLive, err := readLiveCIDRs(repoRoot, downloader)
		if err != nil {
			return Report{}, err
		}
		local, err := queryLocalCIDRs(db, downloaderQueries)
		if err != nil {
			return Report{}, fmt.Errorf("%s: %w", downloader, err)
		}
		report.Services = append(report.Services, diffService(downloader, live, local, invalidLive))
	}

	sort.Slice(report.Services, func(i, j int) bool {
		if report.Services[i].LiveOnlyCount != report.Services[j].LiveOnlyCount {
			return report.Services[i].LiveOnlyCount > report.Services[j].LiveOnlyCount
		}
		if report.Services[i].InvalidLiveCount != report.Services[j].InvalidLiveCount {
			return report.Services[i].InvalidLiveCount > report.Services[j].InvalidLiveCount
		}
		return report.Services[i].Downloader < report.Services[j].Downloader
	})
	return report, nil
}

func WriteJSON(path string, report Report) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal comparison JSON: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create comparison JSON dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write comparison JSON: %w", err)
	}
	return nil
}

func WriteMarkdown(path string, report Report) error {
	var b strings.Builder
	b.WriteString("# Local Routing Index Comparison\n\n")
	b.WriteString("| downloader | status | live | local | live only | local only | invalid live |\n")
	b.WriteString("| --- | --- | ---: | ---: | ---: | ---: | ---: |\n")
	for _, service := range report.Services {
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %d | %d | %d |\n",
			service.Downloader,
			service.Status,
			service.LiveCount,
			service.LocalCount,
			service.LiveOnlyCount,
			service.LocalOnlyCount,
			service.InvalidLiveCount,
		)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create comparison markdown dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write comparison markdown: %w", err)
	}
	return nil
}

func queryLocalCIDRs(db *store.DB, queries []config.Query) (map[string]struct{}, error) {
	local := map[string]struct{}{}
	for _, q := range queries {
		sources := sourceFiltersFromMirrors(q.Mirrors)
		for _, family := range []int{4, 6} {
			var prefixes []string
			var err error
			switch q.Kind {
			case "origin":
				prefixes, err = routequery.ByOrigin(db, q.Value, family, sources)
			case "mnt-by":
				prefixes, err = routequery.ByMaintainer(db, q.Value, family, sources)
			default:
				return nil, fmt.Errorf("unsupported query kind %q", q.Kind)
			}
			if err != nil {
				return nil, err
			}
			for _, prefix := range prefixes {
				normalized, err := normalizeCIDR(prefix)
				if err != nil {
					return nil, err
				}
				local[normalized] = struct{}{}
			}
		}
	}
	return local, nil
}

func readLiveCIDRs(repoRoot string, downloader string) (map[string]struct{}, []string, error) {
	live := map[string]struct{}{}
	var invalid []string
	for _, name := range []string{"ipv4.txt", "ipv6.txt"} {
		path := filepath.Join(repoRoot, downloader, name)
		label := filepath.ToSlash(filepath.Join(downloader, name))
		fileInvalid, err := readCIDRFile(path, label, live)
		if err != nil {
			return nil, nil, err
		}
		invalid = append(invalid, fileInvalid...)
	}
	return live, invalid, nil
}

func readCIDRFile(path string, label string, out map[string]struct{}) ([]string, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open live CIDR file %s: %w", path, err)
	}
	defer file.Close()

	var invalid []string
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		prefix, err := normalizeCIDR(strings.Fields(line)[0])
		if err != nil {
			invalid = append(invalid, fmt.Sprintf("%s:%d: %s", label, lineNo, line))
			continue
		}
		out[prefix] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read live CIDR file %s: %w", path, err)
	}
	return invalid, nil
}

func diffService(downloader string, live map[string]struct{}, local map[string]struct{}, invalidLive []string) ServiceReport {
	localOnly := difference(local, live)
	liveOnly := difference(live, local)
	return ServiceReport{
		Downloader:       downloader,
		Status:           statusForDiff(len(live), len(local), len(liveOnly), len(localOnly), len(invalidLive)),
		LiveCount:        len(live),
		LocalCount:       len(local),
		InvalidLiveCount: len(invalidLive),
		LocalOnlyCount:   len(localOnly),
		LiveOnlyCount:    len(liveOnly),
		InvalidLive:      invalidLive,
		LocalOnly:        localOnly,
		LiveOnly:         liveOnly,
	}
}

func difference(left map[string]struct{}, right map[string]struct{}) []string {
	var values []string
	for value := range left {
		if _, ok := right[value]; ok {
			continue
		}
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func statusForDiff(liveCount int, localCount int, liveOnlyCount int, localOnlyCount int, invalidLiveCount int) string {
	switch {
	case liveOnlyCount > 0:
		return "live_only_missing"
	case invalidLiveCount > 0:
		return "invalid_live_lines"
	case liveCount == localCount && localOnlyCount == 0:
		return "equal"
	case localOnlyCount > 0:
		return "local_includes_live"
	default:
		return "empty"
	}
}

func normalizeCIDR(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	prefix, err := netip.ParsePrefix(trimmed)
	if err != nil {
		addr, addrErr := netip.ParseAddr(trimmed)
		if addrErr != nil {
			return "", fmt.Errorf("parse CIDR %q: %w", value, err)
		}
		if addr.Is4() {
			return netip.PrefixFrom(addr, 32).String(), nil
		}
		return netip.PrefixFrom(addr, 128).String(), nil
	}
	return prefix.Masked().String(), nil
}

func sourceFiltersFromMirrors(mirrors []string) []string {
	if len(mirrors) == 0 {
		return nil
	}
	var sources []string
	for _, mirror := range mirrors {
		switch mirror {
		case "ris":
			sources = append(sources, "ris", "ris_ipv4", "ris_ipv6")
		case "ripe":
			sources = append(sources, "ripe", "ripe_route", "ripe_route6", "ripe_inetnum", "ripe_inet6num")
		case "radb":
			sources = append(sources, "radb")
		case "ntt":
			sources = append(sources, "ntt", "nttcom", "nttcom_route", "nttcom_route6")
		case "rogers":
			sources = append(sources, "rogers", "rogerstelecom")
		case "bgp_net_br":
			sources = append(sources, "bgp_net_br", "bgp.net.br")
		default:
			sources = append(sources, mirror)
		}
	}
	return sources
}
