package store

import (
	"path/filepath"
	"testing"

	"github.com/antonme/ipranges/irrindex/internal/ris"
	"github.com/antonme/ipranges/irrindex/internal/rpsl"
)

func TestOpenAndInsertSources(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	err = db.InsertRPSLSource("ripe_route", []rpsl.Object{{
		Type:   "route",
		Prefix: "23.32.0.0/11",
		Attrs: map[string][]string{
			"origin": {"AS16625"},
			"mnt-by": {"AKAMAI-MNT"},
		},
	}})
	if err != nil {
		t.Fatalf("InsertRPSLSource returned error: %v", err)
	}

	err = db.InsertRISSource("ris_ipv4", []ris.Record{{
		ASN:       "AS16625",
		Prefix:    "23.32.0.0/11",
		PeerCount: 12,
	}, {
		ASN:       "AS64073",
		Prefix:    "0.0.0.0/0",
		PeerCount: 1,
		IsDefault: true,
	}}, 4)
	if err != nil {
		t.Fatalf("InsertRISSource returned error: %v", err)
	}

	if got := db.Stats().DroppedDefaults; got != 1 {
		t.Fatalf("DroppedDefaults = %d, want 1", got)
	}
}
