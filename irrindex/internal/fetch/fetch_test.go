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
