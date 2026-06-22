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

func TestInsertRPSLSourceKeepsInetnumRangeWithoutExpansion(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	err = db.InsertRPSLSource("ripe_inetnum", []rpsl.Object{{
		Type:    "inetnum",
		StartIP: "10.0.0.1",
		EndIP:   "10.0.0.254",
		Attrs: map[string][]string{
			"mnt-by": {"RIPE-NCC-HM-MNT"},
		},
	}})
	if err != nil {
		t.Fatalf("InsertRPSLSource returned error: %v", err)
	}

	var count int
	if err := db.SQL().QueryRow(`SELECT COUNT(*) FROM objects WHERE object_type = 'inetnum'`).Scan(&count); err != nil {
		t.Fatalf("count inetnum objects: %v", err)
	}
	if count != 1 {
		t.Fatalf("inetnum object count = %d, want 1 unexpanded range", count)
	}

	var prefix any
	if err := db.SQL().QueryRow(`SELECT prefix FROM objects WHERE object_type = 'inetnum'`).Scan(&prefix); err != nil {
		t.Fatalf("select inetnum prefix: %v", err)
	}
	if prefix != nil {
		t.Fatalf("inetnum prefix = %#v, want NULL", prefix)
	}
}

func TestInsertRPSLSourceNormalizesLeadingZeroIPv4Route(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	err = db.InsertRPSLSource("arin_irr", []rpsl.Object{{
		Type:   "route",
		Prefix: "064.006.160.000/19",
		Attrs: map[string][]string{
			"origin": {"AS64500"},
		},
	}})
	if err != nil {
		t.Fatalf("InsertRPSLSource returned error: %v", err)
	}

	var prefix string
	if err := db.SQL().QueryRow(`SELECT prefix FROM objects`).Scan(&prefix); err != nil {
		t.Fatalf("select prefix: %v", err)
	}
	if prefix != "64.6.160.0/19" {
		t.Fatalf("prefix = %q, want 64.6.160.0/19", prefix)
	}
}
