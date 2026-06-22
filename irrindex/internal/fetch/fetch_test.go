package fetch

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/antonme/ipranges/irrindex/internal/config"
)

func TestFetchAllDownloadsSourcesAndRecordsErrors(t *testing.T) {
	payload := gzipBytes(t, "route: 203.0.113.0/24\norigin: AS64496\n")
	var okHits atomic.Int32
	var failHits atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok.gz":
			okHits.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(payload)
		case "/fail.gz":
			failHits.Add(1)
			http.Error(w, "upstream failure", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	manifest, err := FetchAll(config.SourcesConfig{Sources: []config.Source{
		{Name: "ripe_route", URL: server.URL + "/ok.gz", Format: "rpsl"},
		{Name: "broken", URL: server.URL + "/fail.gz", Format: "rpsl"},
	}}, cacheDir)
	if err != nil {
		t.Fatalf("FetchAll returned error: %v", err)
	}

	if okHits.Load() != 1 {
		t.Fatalf("ok source hits = %d, want 1", okHits.Load())
	}
	if failHits.Load() != 3 {
		t.Fatalf("failed source hits = %d, want 3 retries", failHits.Load())
	}
	if len(manifest.Sources) != 2 {
		t.Fatalf("manifest source count = %d, want 2", len(manifest.Sources))
	}

	ok := manifest.Sources[0]
	if !ok.OK {
		t.Fatalf("ok source recorded failure: %+v", ok)
	}
	if ok.Name != "ripe_route" {
		t.Fatalf("ok source name = %q", ok.Name)
	}
	if ok.SHA256 != fmt.Sprintf("%x", sha256.Sum256(payload)) {
		t.Fatalf("sha256 = %q", ok.SHA256)
	}
	if ok.SizeBytes != int64(len(payload)) {
		t.Fatalf("size = %d, want %d", ok.SizeBytes, len(payload))
	}
	gotPayload, err := os.ReadFile(filepath.Join(cacheDir, "ripe_route.gz"))
	if err != nil {
		t.Fatalf("read cached payload: %v", err)
	}
	if !bytes.Equal(gotPayload, payload) {
		t.Fatal("cached payload differs from compressed upstream bytes")
	}

	broken := manifest.Sources[1]
	if broken.OK {
		t.Fatalf("broken source recorded success: %+v", broken)
	}
	if broken.Error == "" {
		t.Fatalf("broken source error is empty: %+v", broken)
	}
}

func TestFetchAllDownloadsFTPSourcesWithCurl(t *testing.T) {
	payload := gzipBytes(t, "route: 198.51.100.0/24\norigin: AS64496\n")
	dir := t.TempDir()
	payloadPath := filepath.Join(dir, "payload.gz")
	if err := os.WriteFile(payloadPath, payload, 0o644); err != nil {
		t.Fatalf("write fake curl payload: %v", err)
	}

	curlPath := filepath.Join(dir, "curl")
	curlScript := `#!/bin/sh
set -eu
out=""
url=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output|-o)
      out="$2"
      shift 2
      ;;
    -*)
      shift
      ;;
    *)
      url="$1"
      shift
      ;;
  esac
done
if [ "$url" != "ftp://example.test/radb.db.gz" ]; then
  echo "unexpected url: $url" >&2
  exit 7
fi
cp "$IRRINDEX_FAKE_CURL_PAYLOAD" "$out"
`
	if err := os.WriteFile(curlPath, []byte(curlScript), 0o755); err != nil {
		t.Fatalf("write fake curl: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("IRRINDEX_FAKE_CURL_PAYLOAD", payloadPath)

	cacheDir := filepath.Join(dir, "cache")
	manifest, err := FetchAll(config.SourcesConfig{Sources: []config.Source{{
		Name:   "radb",
		URL:    "ftp://example.test/radb.db.gz",
		Format: "rpsl",
	}}}, cacheDir)
	if err != nil {
		t.Fatalf("FetchAll returned error: %v", err)
	}
	if len(manifest.Sources) != 1 || !manifest.Sources[0].OK {
		t.Fatalf("manifest = %+v, want successful FTP source", manifest)
	}

	got, err := os.ReadFile(filepath.Join(cacheDir, "radb.gz"))
	if err != nil {
		t.Fatalf("read cached FTP payload: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("cached FTP payload differs from fake curl payload")
	}
}

func TestFetchAllErrorsWhenEverySourceFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream failure", http.StatusBadGateway)
	}))
	defer server.Close()

	manifest, err := FetchAll(config.SourcesConfig{Sources: []config.Source{
		{Name: "broken", URL: server.URL + "/fail.gz", Format: "rpsl"},
	}}, t.TempDir())
	if err == nil {
		t.Fatal("FetchAll returned nil error, want all-sources failure")
	}
	if !strings.Contains(err.Error(), "all sources failed") {
		t.Fatalf("error = %v, want all sources failed", err)
	}
	if len(manifest.Sources) != 1 || manifest.Sources[0].OK {
		t.Fatalf("manifest = %+v, want failed source record", manifest)
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
