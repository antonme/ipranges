package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, name, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func TestLoadSources(t *testing.T) {
	path := writeFixture(t, "sources.yaml", `
sources:
  ripe_route:
    url: "https://ftp.ripe.net/ripe/dbase/split/ripe.db.route.gz"
    format: "rpsl"
    object_types: ["route"]
  ris_ipv4:
    url: "https://www.ris.ripe.net/dumps/riswhoisdump.IPv4.gz"
    format: "riswhois"
    family: 4
`)

	cfg, err := LoadSources(path)
	if err != nil {
		t.Fatalf("LoadSources returned error: %v", err)
	}

	if len(cfg.Sources) != 2 {
		t.Fatalf("source count = %d, want 2", len(cfg.Sources))
	}

	ripe := cfg.Sources[0]
	if ripe.Name != "ripe_route" {
		t.Fatalf("first source name = %q, want ripe_route", ripe.Name)
	}
	if ripe.URL != "https://ftp.ripe.net/ripe/dbase/split/ripe.db.route.gz" {
		t.Fatalf("ripe URL = %q", ripe.URL)
	}
	if ripe.Format != "rpsl" {
		t.Fatalf("ripe format = %q, want rpsl", ripe.Format)
	}
	if got := strings.Join(ripe.ObjectTypes, ","); got != "route" {
		t.Fatalf("ripe object types = %q, want route", got)
	}

	ris := cfg.Sources[1]
	if ris.Name != "ris_ipv4" {
		t.Fatalf("second source name = %q, want ris_ipv4", ris.Name)
	}
	if ris.Family != 4 {
		t.Fatalf("ris family = %d, want 4", ris.Family)
	}
}

func TestLoadSourcesRejectsDuplicateNames(t *testing.T) {
	path := writeFixture(t, "sources.yaml", `
sources:
  ripe_route:
    url: "https://example.test/one.gz"
    format: "rpsl"
  ripe_route:
    url: "https://example.test/two.gz"
    format: "rpsl"
`)

	_, err := LoadSources(path)
	if err == nil {
		t.Fatal("LoadSources returned nil error for duplicate source name")
	}
	if !strings.Contains(err.Error(), "duplicate source") {
		t.Fatalf("error = %q, want duplicate source", err)
	}
}

func TestLoadSourcesRejectsEmptyURL(t *testing.T) {
	path := writeFixture(t, "sources.yaml", `
sources:
  empty:
    url: ""
    format: "rpsl"
`)

	_, err := LoadSources(path)
	if err == nil {
		t.Fatal("LoadSources returned nil error for empty URL")
	}
	if !strings.Contains(err.Error(), "empty url") {
		t.Fatalf("error = %q, want empty url", err)
	}
}

func TestLoadQueries(t *testing.T) {
	path := writeFixture(t, "queries.yaml", `
queries:
  - downloader: akamai
    kind: origin
    value: AS16625
    mirrors: ["ris", "radb"]
  - downloader: vpnhosts
    kind: mnt-by
    value: DATACAMP-MNT
    mirrors: ["ripe", "radb"]
`)

	cfg, err := LoadQueries(path)
	if err != nil {
		t.Fatalf("LoadQueries returned error: %v", err)
	}

	if len(cfg.Queries) != 2 {
		t.Fatalf("query count = %d, want 2", len(cfg.Queries))
	}

	q := cfg.Queries[0]
	if q.Downloader != "akamai" {
		t.Fatalf("downloader = %q, want akamai", q.Downloader)
	}
	if q.Kind != "origin" {
		t.Fatalf("kind = %q, want origin", q.Kind)
	}
	if q.Value != "AS16625" {
		t.Fatalf("value = %q, want AS16625", q.Value)
	}
	if got := strings.Join(q.Mirrors, ","); got != "ris,radb" {
		t.Fatalf("mirrors = %q, want ris,radb", got)
	}
}

func TestLoadQueriesRejectsUnsupportedKind(t *testing.T) {
	path := writeFixture(t, "queries.yaml", `
queries:
  - downloader: akamai
    kind: unknown
    value: AS16625
`)

	_, err := LoadQueries(path)
	if err == nil {
		t.Fatal("LoadQueries returned nil error for unsupported kind")
	}
	if !strings.Contains(err.Error(), "unsupported query kind") {
		t.Fatalf("error = %q, want unsupported query kind", err)
	}
}

func TestLoadQueriesRejectsEmptyValue(t *testing.T) {
	path := writeFixture(t, "queries.yaml", `
queries:
  - downloader: akamai
    kind: origin
    value: ""
`)

	_, err := LoadQueries(path)
	if err == nil {
		t.Fatal("LoadQueries returned nil error for empty value")
	}
	if !strings.Contains(err.Error(), "empty query value") {
		t.Fatalf("error = %q, want empty query value", err)
	}
}
