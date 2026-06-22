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

func writeText(t *testing.T, path string, body string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
