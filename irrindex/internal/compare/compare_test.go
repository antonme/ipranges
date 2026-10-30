package compare

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/antonme/ipranges/irrindex/internal/config"
	"github.com/antonme/ipranges/irrindex/internal/rpsl"
	"github.com/antonme/ipranges/irrindex/internal/store"
)

func TestBuildReportLocalIncludesLive(t *testing.T) {
	repoRoot := t.TempDir()
	writeText(t, filepath.Join(repoRoot, "sample", "ipv4.txt"), "10.0.0.0/24\n")

	db, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	err = db.InsertRPSLSource("radb", []rpsl.Object{
		{
			Type:   "route",
			Prefix: "10.0.0.0/24",
			Attrs:  map[string][]string{"origin": {"AS64500"}},
		},
		{
			Type:   "route",
			Prefix: "10.0.1.0/24",
			Attrs:  map[string][]string{"origin": {"AS64500"}},
		},
	})
	if err != nil {
		t.Fatalf("InsertRPSLSource returned error: %v", err)
	}

	report, err := BuildReport(repoRoot, []config.Query{{
		Downloader: "sample",
		Kind:       "origin",
		Value:      "AS64500",
		Mirrors:    []string{"radb"},
	}}, db)
	if err != nil {
		t.Fatalf("BuildReport returned error: %v", err)
	}
	if len(report.Services) != 1 {
		t.Fatalf("service count = %d, want 1", len(report.Services))
	}

	data, err := json.Marshal(report.Services[0])
	if err != nil {
		t.Fatalf("marshal service report: %v", err)
	}
	var got struct {
		Status         string `json:"status"`
		LiveCount      int    `json:"live_count"`
		LocalCount     int    `json:"local_count"`
		LocalOnlyCount int    `json:"local_only_count"`
		LiveOnlyCount  int    `json:"live_only_count"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal service report: %v", err)
	}

	if got.Status != "local_includes_live" {
		t.Fatalf("status = %q, want local_includes_live", got.Status)
	}
	if got.LiveCount != 1 {
		t.Fatalf("live_count = %d, want 1", got.LiveCount)
	}
	if got.LocalCount != 2 {
		t.Fatalf("local_count = %d, want 2", got.LocalCount)
	}
	if got.LocalOnlyCount != 1 {
		t.Fatalf("local_only_count = %d, want 1", got.LocalOnlyCount)
	}
	if got.LiveOnlyCount != 0 {
		t.Fatalf("live_only_count = %d, want 0", got.LiveOnlyCount)
	}
}

func TestBuildReportTreatsLocalSupernetAsCoveringLivePrefixes(t *testing.T) {
	repoRoot := t.TempDir()
	writeText(t, filepath.Join(repoRoot, "sample", "ipv4.txt"), "10.0.0.0/24\n10.0.1.0/24\n")

	db, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	err = db.InsertRPSLSource("radb", []rpsl.Object{{
		Type:   "route",
		Prefix: "10.0.0.0/23",
		Attrs:  map[string][]string{"origin": {"AS64500"}},
	}})
	if err != nil {
		t.Fatalf("InsertRPSLSource returned error: %v", err)
	}

	report, err := BuildReport(repoRoot, []config.Query{{
		Downloader: "sample",
		Kind:       "origin",
		Value:      "AS64500",
		Mirrors:    []string{"radb"},
	}}, db)
	if err != nil {
		t.Fatalf("BuildReport returned error: %v", err)
	}

	service := report.Services[0]
	if service.Status != "equal_coverage" {
		t.Fatalf("status = %q, want equal_coverage", service.Status)
	}
	if service.LiveOnlyCount != 0 {
		t.Fatalf("live_only_count = %d, want 0; live_only = %#v", service.LiveOnlyCount, service.LiveOnly)
	}
	if service.LocalOnlyCount != 0 {
		t.Fatalf("local_only_count = %d, want 0; local_only = %#v", service.LocalOnlyCount, service.LocalOnly)
	}
}

func TestBuildReportReportsOnlyUncoveredLiveAddressSpace(t *testing.T) {
	repoRoot := t.TempDir()
	writeText(t, filepath.Join(repoRoot, "sample", "ipv4.txt"), "10.0.0.0/23\n")

	db, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	err = db.InsertRPSLSource("radb", []rpsl.Object{{
		Type:   "route",
		Prefix: "10.0.0.0/24",
		Attrs:  map[string][]string{"origin": {"AS64500"}},
	}})
	if err != nil {
		t.Fatalf("InsertRPSLSource returned error: %v", err)
	}

	report, err := BuildReport(repoRoot, []config.Query{{
		Downloader: "sample",
		Kind:       "origin",
		Value:      "AS64500",
		Mirrors:    []string{"radb"},
	}}, db)
	if err != nil {
		t.Fatalf("BuildReport returned error: %v", err)
	}

	service := report.Services[0]
	if service.Status != "live_only_missing" {
		t.Fatalf("status = %q, want live_only_missing", service.Status)
	}
	if service.LiveOnlyCount != 1 {
		t.Fatalf("live_only_count = %d, want 1; live_only = %#v", service.LiveOnlyCount, service.LiveOnly)
	}
	if len(service.LiveOnly) != 1 || service.LiveOnly[0] != "10.0.1.0/24" {
		t.Fatalf("live_only = %#v, want [10.0.1.0/24]", service.LiveOnly)
	}
	if service.LocalOnlyCount != 0 {
		t.Fatalf("local_only_count = %d, want 0; local_only = %#v", service.LocalOnlyCount, service.LocalOnly)
	}
}

func TestBuildReportTreatsIPv6SupernetAsCoveringLivePrefixes(t *testing.T) {
	repoRoot := t.TempDir()
	writeText(t, filepath.Join(repoRoot, "sample", "ipv6.txt"), "2001:db8::/127\n2001:db8::2/127\n")

	db, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	err = db.InsertRPSLSource("radb", []rpsl.Object{{
		Type:   "route6",
		Prefix: "2001:db8::/126",
		Attrs:  map[string][]string{"origin": {"AS64500"}},
	}})
	if err != nil {
		t.Fatalf("InsertRPSLSource returned error: %v", err)
	}

	report, err := BuildReport(repoRoot, []config.Query{{
		Downloader: "sample",
		Kind:       "origin",
		Value:      "AS64500",
		Mirrors:    []string{"radb"},
	}}, db)
	if err != nil {
		t.Fatalf("BuildReport returned error: %v", err)
	}

	service := report.Services[0]
	if service.Status != "equal_coverage" {
		t.Fatalf("status = %q, want equal_coverage", service.Status)
	}
	if service.LiveOnlyCount != 0 {
		t.Fatalf("live_only_count = %d, want 0; live_only = %#v", service.LiveOnlyCount, service.LiveOnly)
	}
	if service.LocalOnlyCount != 0 {
		t.Fatalf("local_only_count = %d, want 0; local_only = %#v", service.LocalOnlyCount, service.LocalOnly)
	}
}

func TestBuildReportReportsInvalidLiveLines(t *testing.T) {
	repoRoot := t.TempDir()
	writeText(t, filepath.Join(repoRoot, "sample", "ipv4.txt"), "Something went wrong\n10.0.0.0/24\n")

	db, err := store.Open(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	err = db.InsertRPSLSource("radb", []rpsl.Object{{
		Type:   "route",
		Prefix: "10.0.0.0/24",
		Attrs:  map[string][]string{"origin": {"AS64500"}},
	}})
	if err != nil {
		t.Fatalf("InsertRPSLSource returned error: %v", err)
	}

	report, err := BuildReport(repoRoot, []config.Query{{
		Downloader: "sample",
		Kind:       "origin",
		Value:      "AS64500",
		Mirrors:    []string{"radb"},
	}}, db)
	if err != nil {
		t.Fatalf("BuildReport returned error: %v", err)
	}
	if len(report.Services) != 1 {
		t.Fatalf("service count = %d, want 1", len(report.Services))
	}

	service := report.Services[0]
	if service.Status != "invalid_live_lines" {
		t.Fatalf("status = %q, want invalid_live_lines", service.Status)
	}
	if service.InvalidLiveCount != 1 {
		t.Fatalf("invalid_live_count = %d, want 1", service.InvalidLiveCount)
	}
	if len(service.InvalidLive) != 1 || service.InvalidLive[0] != "sample/ipv4.txt:1: Something went wrong" {
		t.Fatalf("invalid_live = %#v, want recorded invalid line", service.InvalidLive)
	}
}

func TestBuildFileReportIgnoresDefaultRoutes(t *testing.T) {
	referenceRoot := t.TempDir()
	candidateRoot := t.TempDir()
	writeText(t, filepath.Join(referenceRoot, "sample", "ipv4.txt"), "0.0.0.0/0\n10.0.0.0/24\n")
	writeText(t, filepath.Join(referenceRoot, "sample", "ipv6.txt"), "::/0\n2001:db8::/32\n")
	writeText(t, filepath.Join(candidateRoot, "sample", "ipv4.txt"), "10.0.0.0/24\n")
	writeText(t, filepath.Join(candidateRoot, "sample", "ipv6.txt"), "2001:db8::/32\n")

	report, err := BuildFileReport(referenceRoot, candidateRoot, []string{"sample"})
	if err != nil {
		t.Fatalf("BuildFileReport returned error: %v", err)
	}

	service := report.Services[0]
	if service.Status != "equal" {
		t.Fatalf("status = %q, want equal; live_only = %#v", service.Status, service.LiveOnly)
	}
	if service.LiveCount != 2 {
		t.Fatalf("live_count = %d, want 2", service.LiveCount)
	}
	if service.LocalCount != 2 {
		t.Fatalf("local_count = %d, want 2", service.LocalCount)
	}
}

func TestBuildFileReportTreatsNonCanonicalCIDRAsInvalid(t *testing.T) {
	referenceRoot := t.TempDir()
	candidateRoot := t.TempDir()
	writeText(t, filepath.Join(referenceRoot, "sample", "ipv4.txt"), "178.249.213.0/2\n178.249.213.0/24\n")
	writeText(t, filepath.Join(candidateRoot, "sample", "ipv4.txt"), "178.249.213.0/24\n")

	report, err := BuildFileReport(referenceRoot, candidateRoot, []string{"sample"})
	if err != nil {
		t.Fatalf("BuildFileReport returned error: %v", err)
	}

	service := report.Services[0]
	if service.Status != "invalid_live_lines" {
		t.Fatalf("status = %q, want invalid_live_lines; live_only = %#v", service.Status, service.LiveOnly)
	}
	if service.InvalidLiveCount != 1 {
		t.Fatalf("invalid_live_count = %d, want 1", service.InvalidLiveCount)
	}
	if service.LiveOnlyCount != 0 {
		t.Fatalf("live_only_count = %d, want 0; live_only = %#v", service.LiveOnlyCount, service.LiveOnly)
	}
	if len(service.InvalidLive) != 1 || service.InvalidLive[0] != "sample/ipv4.txt:1: 178.249.213.0/2" {
		t.Fatalf("invalid_live = %#v, want non-canonical CIDR recorded", service.InvalidLive)
	}
}

func TestNormalizeCIDRAcceptsBareHostAddresses(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		want  string
	}{
		{name: "ipv4", value: "149.126.75.177", want: "149.126.75.177/32"},
		{name: "ipv6", value: "2001:db8::1", want: "2001:db8::1/128"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeCIDR(tc.value)
			if err != nil {
				t.Fatalf("normalizeCIDR returned error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("normalizeCIDR = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSourceFiltersFromMirrorsIncludesRIPENonAuthForNTT(t *testing.T) {
	got := sourceFiltersFromMirrors([]string{"ntt"})
	for _, want := range []string{"nttcom", "radb", "ripe_nonauth_route", "ripe_nonauth_route6", "rpki", "altdb", "afrinic"} {
		if !hasString(got, want) {
			t.Fatalf("source filters = %#v, want %s", got, want)
		}
	}
	if hasString(got, "reach") {
		t.Fatalf("source filters = %#v, NTT does not mirror REACH", got)
	}
}

func TestSourceFiltersFromMirrorsIncludesAdditionalRADBMirrorDumps(t *testing.T) {
	got := sourceFiltersFromMirrors([]string{"radb"})
	for _, want := range []string{"altdb", "reach", "afrinic", "arin_irr", "ripe_route", "ripe_nonauth_route", "apnic_route", "nttcom", "bgp_net_br"} {
		if !hasString(got, want) {
			t.Fatalf("source filters = %#v, want %s", got, want)
		}
	}
}

func TestSourceFiltersFromMirrorsIncludesRIPENonAuthForRIPE(t *testing.T) {
	got := sourceFiltersFromMirrors([]string{"ripe"})
	for _, want := range []string{"ripe_route", "ripe_nonauth_route", "ripe_nonauth_route6", "ripe_inetnum"} {
		if !hasString(got, want) {
			t.Fatalf("source filters = %#v, want %s", got, want)
		}
	}
}

func writeText(t *testing.T, path string, body string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func hasString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
