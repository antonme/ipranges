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

	code := run([]string{"fetch", "-sources", sourcesPath, "-cache", cacheDir}, &stdout, &stderr)

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

	code := run([]string{"build", "-sources", sourcesPath, "-cache", cacheDir, "-db", dbPath}, &stdout, &stderr)

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

func TestRunKnownSubcommandsAreRecognizedButNotImplemented(t *testing.T) {
	for _, subcommand := range []string{"verify", "compare"} {
		t.Run(subcommand, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := run([]string{subcommand}, &stdout, &stderr)

			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if !strings.Contains(stderr.String(), subcommand+" is not implemented yet") {
				t.Fatalf("stderr = %q, want not implemented message", stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
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
