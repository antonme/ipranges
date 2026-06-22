package query

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/antonme/ipranges/irrindex/internal/ris"
	"github.com/antonme/ipranges/irrindex/internal/rpsl"
	"github.com/antonme/ipranges/irrindex/internal/store"
)

func TestByOriginCombinesRPSLAndRIS(t *testing.T) {
	db := buildTestDB(t)
	defer db.Close()

	got, err := ByOrigin(db, "AS16625", 4, nil)
	if err != nil {
		t.Fatalf("ByOrigin returned error: %v", err)
	}

	want := []string{"23.32.0.0/11", "23.64.0.0/14"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ByOrigin = %#v, want %#v", got, want)
	}
}

func TestByMaintainerIndexesEveryMaintainer(t *testing.T) {
	db := buildTestDB(t)
	defer db.Close()

	for _, maintainer := range []string{"FIRST-MNT", "SECOND-MNT"} {
		got, err := ByMaintainer(db, maintainer, 6, nil)
		if err != nil {
			t.Fatalf("ByMaintainer(%s) returned error: %v", maintainer, err)
		}
		want := []string{"2001:db8::/32"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ByMaintainer(%s) = %#v, want %#v", maintainer, got, want)
		}
	}
}

func TestByMaintainerConvertsInetnumRanges(t *testing.T) {
	db := buildTestDB(t)
	defer db.Close()

	got, err := ByMaintainer(db, "RIPE-NCC-HM-MNT", 4, nil)
	if err != nil {
		t.Fatalf("ByMaintainer returned error: %v", err)
	}
	want := []string{"213.159.160.0/19"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ByMaintainer inetnum = %#v, want %#v", got, want)
	}
}

func TestQueriesDropDefaultRoutesAndUnknownsAreEmpty(t *testing.T) {
	db := buildTestDB(t)
	defer db.Close()

	for _, tc := range []struct {
		name string
		run  func() ([]string, error)
	}{
		{name: "ris default", run: func() ([]string, error) { return ByOrigin(db, "AS64073", 4, nil) }},
		{name: "rpsl default", run: func() ([]string, error) { return ByOrigin(db, "AS64496", 6, nil) }},
		{name: "unknown origin", run: func() ([]string, error) { return ByOrigin(db, "AS65500", 4, nil) }},
		{name: "unknown maintainer", run: func() ([]string, error) { return ByMaintainer(db, "UNKNOWN-MNT", 6, nil) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.run()
			if err != nil {
				t.Fatalf("query returned error: %v", err)
			}
			if len(got) != 0 {
				t.Fatalf("query = %#v, want empty result", got)
			}
		})
	}
}

func TestQueriesFilterBySource(t *testing.T) {
	db := buildTestDB(t)
	defer db.Close()

	got, err := ByOrigin(db, "AS16625", 4, []string{"ripe_route"})
	if err != nil {
		t.Fatalf("ByOrigin returned error: %v", err)
	}
	want := []string{"23.64.0.0/14"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ByOrigin with source filter = %#v, want %#v", got, want)
	}
}

func buildTestDB(t *testing.T) *store.DB {
	t.Helper()

	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	err = db.InsertRPSLSource("ripe_route", []rpsl.Object{
		{
			Type:   "route",
			Prefix: "23.64.0.0/14",
			Attrs: map[string][]string{
				"origin": {"AS16625"},
				"mnt-by": {"AKAMAI-MNT"},
			},
		},
		{
			Type:   "route6",
			Prefix: "2001:db8::/32",
			Attrs: map[string][]string{
				"origin": {"AS65536"},
				"mnt-by": {"FIRST-MNT", "SECOND-MNT"},
			},
		},
		{
			Type:   "route6",
			Prefix: "::/0",
			Attrs: map[string][]string{
				"origin": {"AS64496"},
				"mnt-by": {"DEFAULT-MNT"},
			},
		},
		{
			Type:    "inetnum",
			StartIP: "213.159.160.0",
			EndIP:   "213.159.191.255",
			Attrs: map[string][]string{
				"mnt-by": {"RIPE-NCC-HM-MNT"},
			},
		},
	})
	if err != nil {
		db.Close()
		t.Fatalf("InsertRPSLSource returned error: %v", err)
	}

	err = db.InsertRISSource("ris_ipv4", []ris.Record{
		{ASN: "AS64073", Prefix: "0.0.0.0/0", PeerCount: 1, IsDefault: true},
		{ASN: "AS16625", Prefix: "23.32.0.0/11", PeerCount: 12},
	}, 4)
	if err != nil {
		db.Close()
		t.Fatalf("InsertRISSource returned error: %v", err)
	}

	return db
}
