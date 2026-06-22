package manifest

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExtractQueriesFromDownloaderHelpers(t *testing.T) {
	root := t.TempDir()
	serviceDir := filepath.Join(root, "sample")
	if err := os.MkdirAll(serviceDir, 0o755); err != nil {
		t.Fatalf("create service dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(serviceDir, "downloader.sh"), []byte(`#!/usr/bin/env bash
get_routes() {
    whois -h riswhois.ripe.net -- "-i origin $1" | rg '^route'
    whois -h whois.radb.net -- "-i origin $1" | rg '^route'
}
get_maintained() {
    whois -h whois.ripe.net -- "-i mnt-by $1" | rg '^route'
    whois -h whois.radb.net -- "-i mnt-by $1" | rg '^route'
}

get_routes 'AS16625'
get_routes 'AS-SET-NOT-SUPPORTED'
for asn in AS174 AS1239; do
    get_routes "$asn"
done
get_maintained 'DATACAMP-MNT'
`), 0o644); err != nil {
		t.Fatalf("write downloader: %v", err)
	}

	got, err := ExtractQueries(root)
	if err != nil {
		t.Fatalf("ExtractQueries returned error: %v", err)
	}

	want := []QuerySpec{
		{
			Downloader: "sample",
			Kind:       "mnt-by",
			Value:      "DATACAMP-MNT",
			Hosts:      []string{"whois.ripe.net", "whois.radb.net"},
			Mirrors:    []string{"ripe", "radb"},
		},
		{
			Downloader: "sample",
			Kind:       "origin",
			Value:      "AS1239",
			Hosts:      []string{"riswhois.ripe.net", "whois.radb.net"},
			Mirrors:    []string{"ris", "radb"},
		},
		{
			Downloader: "sample",
			Kind:       "origin",
			Value:      "AS16625",
			Hosts:      []string{"riswhois.ripe.net", "whois.radb.net"},
			Mirrors:    []string{"ris", "radb"},
		},
		{
			Downloader: "sample",
			Kind:       "origin",
			Value:      "AS174",
			Hosts:      []string{"riswhois.ripe.net", "whois.radb.net"},
			Mirrors:    []string{"ris", "radb"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExtractQueries = %#v, want %#v", got, want)
	}
}

func TestExtractHelpersFindsMaintainerHelper(t *testing.T) {
	helpers := extractHelpers(`get_maintained() {
    whois -h whois.ripe.net -- "-i mnt-by $1" | rg '^route'
    whois -h whois.radb.net -- "-i mnt-by $1" | rg '^route'
}
`)

	got, ok := helpers["get_maintained"]
	if !ok {
		t.Fatalf("get_maintained helper missing: %#v", helpers)
	}
	want := helperSpec{
		kind:    "mnt-by",
		hosts:   []string{"whois.ripe.net", "whois.radb.net"},
		mirrors: []string{"ripe", "radb"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("helper = %#v, want %#v", got, want)
	}
}
