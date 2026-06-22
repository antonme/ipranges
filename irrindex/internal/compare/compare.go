package compare

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/bits"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/antonme/ipranges/irrindex/internal/config"
	routequery "github.com/antonme/ipranges/irrindex/internal/query"
	"github.com/antonme/ipranges/irrindex/internal/store"
)

const comparisonScope = "local_query_results_vs_committed_downloader_outputs"

type Report struct {
	ComparisonScope string          `json:"comparison_scope"`
	Services        []ServiceReport `json:"services"`
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

	report := Report{
		ComparisonScope: comparisonScope,
		Services:        make([]ServiceReport, 0, len(byDownloader)),
	}
	for downloader, downloaderQueries := range byDownloader {
		live, invalidLive, err := readLiveCIDRs(repoRoot, downloader)
		if err != nil {
			return Report{}, err
		}
		local, err := queryLocalCIDRs(db, downloaderQueries)
		if err != nil {
			return Report{}, fmt.Errorf("%s: %w", downloader, err)
		}
		serviceReport, err := diffService(downloader, live, local, invalidLive)
		if err != nil {
			return Report{}, fmt.Errorf("%s: %w", downloader, err)
		}
		report.Services = append(report.Services, serviceReport)
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
	b.WriteString("# Local Routing Index Coverage Comparison\n\n")
	b.WriteString("Scope: configured local queries compared with committed downloader outputs. This report uses CIDR coverage, not exact line equality; downloaders may include non-WHOIS feeds outside `queries.yaml`.\n\n")
	b.WriteString("| downloader | status | live | local | live only coverage | local only coverage | invalid live |\n")
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

func diffService(downloader string, live map[string]struct{}, local map[string]struct{}, invalidLive []string) (ServiceReport, error) {
	localOnly, err := coverageDifference(local, live)
	if err != nil {
		return ServiceReport{}, fmt.Errorf("compare local-only coverage: %w", err)
	}
	liveOnly, err := coverageDifference(live, local)
	if err != nil {
		return ServiceReport{}, fmt.Errorf("compare live-only coverage: %w", err)
	}
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
	}, nil
}

type uint128 struct {
	hi uint64
	lo uint64
}

type ipRange struct {
	family int
	start  uint128
	end    uint128
}

func coverageDifference(left map[string]struct{}, right map[string]struct{}) ([]string, error) {
	leftRanges, err := prefixesToRanges(left)
	if err != nil {
		return nil, err
	}
	rightRanges, err := prefixesToRanges(right)
	if err != nil {
		return nil, err
	}

	diff := subtractRanges(mergeRanges(leftRanges), mergeRanges(rightRanges))
	return rangesToPrefixes(diff)
}

func prefixesToRanges(values map[string]struct{}) ([]ipRange, error) {
	ranges := make([]ipRange, 0, len(values))
	for value := range values {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return nil, fmt.Errorf("parse normalized CIDR %q: %w", value, err)
		}
		ranges = append(ranges, prefixToRange(prefix.Masked()))
	}
	return ranges, nil
}

func prefixToRange(prefix netip.Prefix) ipRange {
	start, family := addrToUint128(prefix.Addr())
	hostBits := family - prefix.Bits()
	end := addBlockEnd(start, hostBits, family)
	return ipRange{family: family, start: start, end: end}
}

func mergeRanges(ranges []ipRange) []ipRange {
	if len(ranges) == 0 {
		return nil
	}
	sortRanges(ranges)

	merged := []ipRange{ranges[0]}
	for _, r := range ranges[1:] {
		last := &merged[len(merged)-1]
		if last.family == r.family && rangeTouchesOrOverlaps(*last, r) {
			if compareUint128(r.end, last.end) > 0 {
				last.end = r.end
			}
			continue
		}
		merged = append(merged, r)
	}
	return merged
}

func subtractRanges(left []ipRange, right []ipRange) []ipRange {
	var out []ipRange
	rightIndex := 0
	for _, l := range left {
		for rightIndex < len(right) && rangeBefore(right[rightIndex], l) {
			rightIndex++
		}

		cursor := l.start
		consumed := false
		for i := rightIndex; i < len(right); i++ {
			r := right[i]
			if r.family > l.family || (r.family == l.family && compareUint128(r.start, l.end) > 0) {
				break
			}
			if r.family < l.family || compareUint128(r.end, cursor) < 0 {
				continue
			}

			if compareUint128(r.start, cursor) > 0 {
				out = append(out, ipRange{family: l.family, start: cursor, end: decrementUint128(r.start)})
			}
			if compareUint128(r.end, l.end) >= 0 {
				consumed = true
				break
			}
			cursor = incrementUint128(r.end)
		}

		if !consumed && compareUint128(cursor, l.end) <= 0 {
			out = append(out, ipRange{family: l.family, start: cursor, end: l.end})
		}
	}
	return out
}

func rangesToPrefixes(ranges []ipRange) ([]string, error) {
	sortRanges(ranges)

	var prefixes []string
	for _, r := range ranges {
		current := r.start
		for compareUint128(current, r.end) <= 0 {
			hostBits := largestCIDRHostBits(current, r.end, r.family)
			prefixBits := r.family - hostBits
			addr, err := uint128ToAddr(current, r.family)
			if err != nil {
				return nil, err
			}
			prefixes = append(prefixes, netip.PrefixFrom(addr, prefixBits).String())

			blockEnd := addBlockEnd(current, hostBits, r.family)
			if compareUint128(blockEnd, r.end) >= 0 {
				break
			}
			current = incrementUint128(blockEnd)
		}
	}
	return prefixes, nil
}

func largestCIDRHostBits(start uint128, end uint128, family int) int {
	hostBits := trailingZeroBits(start)
	if hostBits > family {
		hostBits = family
	}
	for hostBits > 0 && compareUint128(addBlockEnd(start, hostBits, family), end) > 0 {
		hostBits--
	}
	return hostBits
}

func sortRanges(ranges []ipRange) {
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].family != ranges[j].family {
			return ranges[i].family < ranges[j].family
		}
		if cmp := compareUint128(ranges[i].start, ranges[j].start); cmp != 0 {
			return cmp < 0
		}
		return compareUint128(ranges[i].end, ranges[j].end) < 0
	})
}

func rangeTouchesOrOverlaps(left ipRange, right ipRange) bool {
	if compareUint128(right.start, left.end) <= 0 {
		return true
	}
	if compareUint128(left.end, maxValue(left.family)) == 0 {
		return false
	}
	return compareUint128(right.start, incrementUint128(left.end)) <= 0
}

func rangeBefore(left ipRange, right ipRange) bool {
	if left.family != right.family {
		return left.family < right.family
	}
	return compareUint128(left.end, right.start) < 0
}

func addrToUint128(addr netip.Addr) (uint128, int) {
	if addr.Is4() {
		bytes := addr.As4()
		return uint128{lo: uint64(binary.BigEndian.Uint32(bytes[:]))}, 32
	}

	bytes := addr.As16()
	return uint128{
		hi: binary.BigEndian.Uint64(bytes[:8]),
		lo: binary.BigEndian.Uint64(bytes[8:]),
	}, 128
}

func uint128ToAddr(value uint128, family int) (netip.Addr, error) {
	switch family {
	case 32:
		if value.hi != 0 || value.lo > math.MaxUint32 {
			return netip.Addr{}, fmt.Errorf("IPv4 value out of range: %#v", value)
		}
		var bytes [4]byte
		binary.BigEndian.PutUint32(bytes[:], uint32(value.lo))
		return netip.AddrFrom4(bytes), nil
	case 128:
		var bytes [16]byte
		binary.BigEndian.PutUint64(bytes[:8], value.hi)
		binary.BigEndian.PutUint64(bytes[8:], value.lo)
		return netip.AddrFrom16(bytes), nil
	default:
		return netip.Addr{}, fmt.Errorf("unsupported address width %d", family)
	}
}

func addBlockEnd(start uint128, hostBits int, family int) uint128 {
	if hostBits >= family && compareUint128(start, zeroValue()) == 0 {
		return maxValue(family)
	}
	if hostBits == 0 {
		return start
	}
	return decrementUint128(addPowerOfTwo(start, hostBits))
}

func addPowerOfTwo(value uint128, exponent int) uint128 {
	if exponent < 64 {
		add := uint64(1) << exponent
		lo := value.lo + add
		if lo < value.lo {
			value.hi++
		}
		value.lo = lo
		return value
	}
	value.hi += uint64(1) << (exponent - 64)
	return value
}

func incrementUint128(value uint128) uint128 {
	value.lo++
	if value.lo == 0 {
		value.hi++
	}
	return value
}

func decrementUint128(value uint128) uint128 {
	if value.lo == 0 {
		value.hi--
		value.lo = math.MaxUint64
		return value
	}
	value.lo--
	return value
}

func compareUint128(left uint128, right uint128) int {
	switch {
	case left.hi < right.hi:
		return -1
	case left.hi > right.hi:
		return 1
	case left.lo < right.lo:
		return -1
	case left.lo > right.lo:
		return 1
	default:
		return 0
	}
}

func trailingZeroBits(value uint128) int {
	switch {
	case value.lo != 0:
		return bits.TrailingZeros64(value.lo)
	case value.hi != 0:
		return 64 + bits.TrailingZeros64(value.hi)
	default:
		return 128
	}
}

func zeroValue() uint128 {
	return uint128{}
}

func maxValue(family int) uint128 {
	if family == 32 {
		return uint128{lo: math.MaxUint32}
	}
	return uint128{hi: math.MaxUint64, lo: math.MaxUint64}
}

func statusForDiff(liveCount int, localCount int, liveOnlyCount int, localOnlyCount int, invalidLiveCount int) string {
	switch {
	case liveOnlyCount > 0:
		return "live_only_missing"
	case invalidLiveCount > 0:
		return "invalid_live_lines"
	case liveCount == localCount && localOnlyCount == 0:
		return "equal"
	case liveCount > 0 && localCount > 0 && localOnlyCount == 0:
		return "equal_coverage"
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
