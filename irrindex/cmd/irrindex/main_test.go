package main

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/antonme/ipranges/irrindex/internal/config"
	"github.com/antonme/ipranges/irrindex/internal/query"
	"github.com/antonme/ipranges/irrindex/internal/ris"
	"github.com/antonme/ipranges/irrindex/internal/rpsl"
	"github.com/antonme/ipranges/irrindex/internal/store"
)

func TestRunShowsUsageWithoutSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "Usage: irrindex") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunRejectsUnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"bogus"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown subcommand") {
		t.Fatalf("stderr = %q, want unknown subcommand", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Usage: irrindex") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestRunFetchDownloadsConfiguredSources(t *testing.T) {
	payload := gzipBytes(t, "route: 203.0.113.0/24\norigin: AS64496\n")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	dir := t.TempDir()
	sourcesPath := writeFile(t, dir, "sources.yaml", `
sources:
  ripe_route:
    url: "`+server.URL+`/ripe.db.route.gz"
    format: "rpsl"
`)
	cacheDir := filepath.Join(dir, "cache")
	var stdout, stderr bytes.Buffer

	manifestPath := filepath.Join(dir, "source-manifest.json")
	code := run([]string{"fetch", "--config", sourcesPath, "--cache-dir", cacheDir, "--manifest", manifestPath}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ripe_route ok") {
		t.Fatalf("stdout = %q, want source success", stdout.String())
	}
	got, err := os.ReadFile(filepath.Join(cacheDir, "ripe_route.gz"))
	if err != nil {
		t.Fatalf("read cached source: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("cached source differs from upstream gzip bytes")
	}
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read source manifest: %v", err)
	}
	if !strings.Contains(string(manifestData), `"name": "ripe_route"`) {
		t.Fatalf("manifest = %s", manifestData)
	}
}

func TestRunBuildCreatesQueryableDatabase(t *testing.T) {
	dir := t.TempDir()
	sourcesPath := writeFile(t, dir, "sources.yaml", `
sources:
  ripe_route:
    url: "https://example.invalid/ripe.db.route.gz"
    format: "rpsl"
  ris_ipv4:
    url: "https://example.invalid/riswhoisdump.IPv4.gz"
    format: "riswhois"
    family: 4
`)
	cacheDir := filepath.Join(dir, "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("create cache dir: %v", err)
	}
	writeBytes(t, filepath.Join(cacheDir, "ripe_route.gz"), gzipBytes(t, `route:          23.64.0.0/14
origin:         AS16625
mnt-by:         AKAMAI-MNT
source:         RIPE
`))
	writeBytes(t, filepath.Join(cacheDir, "ris_ipv4.gz"), gzipBytes(t, "64073\t0.0.0.0/0\t1\n16625\t23.32.0.0/11\t12\n"))

	dbPath := filepath.Join(dir, "index.db")
	var stdout, stderr bytes.Buffer

	code := run([]string{"build", "--config", sourcesPath, "--cache-dir", cacheDir, "--db", dbPath}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ripe_route ok") || !strings.Contains(stdout.String(), "ris_ipv4 ok") {
		t.Fatalf("stdout = %q, want build source summaries", stdout.String())
	}

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open built db: %v", err)
	}
	defer db.Close()

	got, err := query.ByOrigin(db, "AS16625", 4, nil)
	if err != nil {
		t.Fatalf("ByOrigin returned error: %v", err)
	}
	want := []string{"23.32.0.0/11", "23.64.0.0/14"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ByOrigin = %#v, want %#v", got, want)
	}

	got, err = query.ByOrigin(db, "AS64073", 4, nil)
	if err != nil {
		t.Fatalf("ByOrigin default returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("default route query = %#v, want empty", got)
	}
}

func TestRunVerifyCommand(t *testing.T) {
	dbPath := buildCommandDB(t)
	var stdout, stderr bytes.Buffer

	code := run([]string{"verify", "--db", dbPath}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "verify ok") {
		t.Fatalf("stdout = %q, want verify ok", stdout.String())
	}
}

func TestRunQueryCommands(t *testing.T) {
	dbPath := buildCommandDB(t)

	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{
			name: "origin",
			args: []string{"query", "origin", "AS16625", "--db", dbPath, "--family", "4"},
			want: "23.32.0.0/11\n23.64.0.0/14\n",
		},
		{
			name: "mnt-by",
			args: []string{"query", "mnt-by", "DATACAMP-MNT", "--db", dbPath, "--family", "6"},
			want: "2001:db8::/32\n",
		},
		{
			name: "empty",
			args: []string{"query", "origin", "AS65500", "--db", dbPath, "--family", "4"},
			want: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := run(tc.args, &stdout, &stderr)

			if code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
			}
			if stdout.String() != tc.want {
				t.Fatalf("stdout = %q, want %q", stdout.String(), tc.want)
			}
		})
	}
}

func TestRunWhoisShimCommands(t *testing.T) {
	dbPath := buildCommandDB(t)

	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{
			name: "radb origin",
			args: []string{"whois-shim", "--db", dbPath, "-h", "whois.radb.net", "--", "-i origin AS16625"},
			want: "route:          23.64.0.0/14\n",
		},
		{
			name: "ripe mnt-by",
			args: []string{"whois-shim", "--db", dbPath, "-h", "whois.ripe.net", "--", "-i mnt-by DATACAMP-MNT"},
			want: "route6:         2001:db8::/32\n",
		},
		{
			name: "empty",
			args: []string{"whois-shim", "--db", dbPath, "-h", "whois.radb.net", "--", "-i origin AS65500"},
			want: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := run(tc.args, &stdout, &stderr)

			if code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
			}
			if stdout.String() != tc.want {
				t.Fatalf("stdout = %q, want %q", stdout.String(), tc.want)
			}
		})
	}
}

func TestRunExtractQueriesWritesManifest(t *testing.T) {
	dir := t.TempDir()
	serviceDir := filepath.Join(dir, "sample")
	if err := os.MkdirAll(serviceDir, 0o755); err != nil {
		t.Fatalf("create service dir: %v", err)
	}
	writeFile(t, serviceDir, "downloader.sh", `get_routes() {
    whois -h riswhois.ripe.net -- "-i origin $1" | rg '^route'
    whois -h whois.radb.net -- "-i origin $1" | rg '^route'
}
get_routes 'AS16625'
`)
	outPath := filepath.Join(dir, "queries.yaml")
	var stdout, stderr bytes.Buffer

	code := run([]string{"extract-queries", "--repo", dir, "--out", outPath}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	cfg, err := config.LoadQueries(outPath)
	if err != nil {
		t.Fatalf("LoadQueries generated manifest: %v", err)
	}
	if len(cfg.Queries) != 1 {
		t.Fatalf("query count = %d, want 1", len(cfg.Queries))
	}
	got := cfg.Queries[0]
	if got.Downloader != "sample" || got.Kind != "origin" || got.Value != "AS16625" {
		t.Fatalf("query = %+v", got)
	}
	if !reflect.DeepEqual(got.Mirrors, []string{"ris", "radb"}) {
		t.Fatalf("mirrors = %#v", got.Mirrors)
	}
}

func TestRunCompareWritesReports(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sample"), 0o755); err != nil {
		t.Fatalf("create sample dir: %v", err)
	}
	writeFile(t, filepath.Join(dir, "sample"), "ipv4.txt", "10.0.0.0/24\n")
	queriesPath := writeFile(t, dir, "queries.yaml", `queries:
  - downloader: sample
    kind: origin
    value: AS64500
    mirrors: ["radb"]
`)
	dbPath := filepath.Join(dir, "index.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	err = db.InsertRPSLSource("radb", []rpsl.Object{
		{Type: "route", Prefix: "10.0.0.0/24", Attrs: map[string][]string{"origin": {"AS64500"}}},
		{Type: "route", Prefix: "10.0.1.0/24", Attrs: map[string][]string{"origin": {"AS64500"}}},
	})
	if err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	var stdout, stderr bytes.Buffer

	outDir := filepath.Join(dir, "report")
	code := run([]string{"compare", "--live-root", dir, "--queries", queriesPath, "--db", dbPath, "--out", outDir}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	jsonData, err := os.ReadFile(filepath.Join(outDir, "comparison.json"))
	if err != nil {
		t.Fatalf("read comparison json: %v", err)
	}
	if !strings.Contains(string(jsonData), `"status": "local_includes_live"`) {
		t.Fatalf("comparison json = %s", jsonData)
	}
	mdData, err := os.ReadFile(filepath.Join(outDir, "comparison.md"))
	if err != nil {
		t.Fatalf("read comparison markdown: %v", err)
	}
	if !strings.Contains(string(mdData), "| sample | local_includes_live | 1 | 2 | 0 | 1 |") {
		t.Fatalf("comparison markdown = %s", mdData)
	}
}

func TestRunManifestSummarize(t *testing.T) {
	dir := t.TempDir()
	manifestPath := writeFile(t, dir, "snapshots.json", `[
  {"name":"daily-00","cadence":"daily","created_at":"2026-06-22T00:00:00Z"},
  {"name":"daily-01","cadence":"daily","created_at":"2026-06-21T00:00:00Z"},
  {"name":"daily-02","cadence":"daily","created_at":"2026-06-20T00:00:00Z"},
  {"name":"daily-03","cadence":"daily","created_at":"2026-06-19T00:00:00Z"},
  {"name":"daily-04","cadence":"daily","created_at":"2026-06-18T00:00:00Z"},
  {"name":"daily-05","cadence":"daily","created_at":"2026-06-17T00:00:00Z"},
  {"name":"daily-06","cadence":"daily","created_at":"2026-06-16T00:00:00Z"},
  {"name":"daily-07","cadence":"daily","created_at":"2026-06-15T00:00:00Z"},
  {"name":"daily-08","cadence":"daily","created_at":"2026-06-14T00:00:00Z"},
  {"name":"daily-09","cadence":"daily","created_at":"2026-06-13T00:00:00Z"},
  {"name":"daily-10","cadence":"daily","created_at":"2026-06-12T00:00:00Z"},
  {"name":"daily-11","cadence":"daily","created_at":"2026-06-11T00:00:00Z"},
  {"name":"daily-12","cadence":"daily","created_at":"2026-06-10T00:00:00Z"},
  {"name":"daily-13","cadence":"daily","created_at":"2026-06-09T00:00:00Z"},
  {"name":"daily-14","cadence":"daily","created_at":"2026-06-08T00:00:00Z"}
]`)
	var stdout, stderr bytes.Buffer

	code := run([]string{"manifest", "summarize", "--manifest", manifestPath}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "keep=14 prune=1") {
		t.Fatalf("stdout = %q, want keep/prune summary", stdout.String())
	}
}

func buildCommandDB(t *testing.T) string {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	err = db.InsertRPSLSource("radb", []rpsl.Object{{
		Type:   "route",
		Prefix: "23.64.0.0/14",
		Attrs: map[string][]string{
			"origin": {"AS16625"},
			"mnt-by": {"AKAMAI-MNT"},
		},
	}})
	if err != nil {
		t.Fatalf("insert radb source: %v", err)
	}

	err = db.InsertRPSLSource("ripe_route6", []rpsl.Object{{
		Type:   "route6",
		Prefix: "2001:db8::/32",
		Attrs: map[string][]string{
			"origin": {"AS65536"},
			"mnt-by": {"DATACAMP-MNT"},
		},
	}})
	if err != nil {
		t.Fatalf("insert ripe source: %v", err)
	}

	err = db.InsertRISSource("ris_ipv4", []ris.Record{{
		ASN:       "AS16625",
		Prefix:    "23.32.0.0/11",
		PeerCount: 12,
	}}, 4)
	if err != nil {
		t.Fatalf("insert ris source: %v", err)
	}

	return dbPath
}

func writeFile(t *testing.T, dir string, name string, body string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func writeBytes(t *testing.T, path string, body []byte) {
	t.Helper()

	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func gzipBytes(t *testing.T, body string) []byte {
	t.Helper()

	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write([]byte(body)); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}
