package rpsl

import (
	"os"
	"strings"
	"testing"
)

func TestParseRoutePreservesRepeatedAttributes(t *testing.T) {
	file, err := os.Open("../../testdata/rpsl/ripe-route.txt")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer file.Close()

	objects, err := Parse(file)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if len(objects) != 1 {
		t.Fatalf("object count = %d, want 1", len(objects))
	}
	obj := objects[0]
	if obj.Type != "route" {
		t.Fatalf("Type = %q, want route", obj.Type)
	}
	if obj.Prefix != "193.254.30.0/24" {
		t.Fatalf("Prefix = %q, want 193.254.30.0/24", obj.Prefix)
	}
	if got := strings.Join(obj.Attrs["origin"], ","); got != "AS12726" {
		t.Fatalf("origin attrs = %q, want AS12726", got)
	}
	if got := strings.Join(obj.Attrs["mnt-by"], ","); got != "AS12312-MNT,DE-COLT-MNT,COLT-UK" {
		t.Fatalf("mnt-by attrs = %q", got)
	}
	if got := strings.Join(obj.Attrs["source"], ","); got != "RIPE" {
		t.Fatalf("source attrs = %q, want RIPE", got)
	}
}

func TestParseRoute6PreservesRepeatedMaintainers(t *testing.T) {
	file, err := os.Open("../../testdata/rpsl/route6-multi-mnt.txt")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer file.Close()

	objects, err := Parse(file)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if len(objects) != 1 {
		t.Fatalf("object count = %d, want 1", len(objects))
	}
	obj := objects[0]
	if obj.Type != "route6" {
		t.Fatalf("Type = %q, want route6", obj.Type)
	}
	if obj.Prefix != "2001:db8::/32" {
		t.Fatalf("Prefix = %q, want 2001:db8::/32", obj.Prefix)
	}
	if got := strings.Join(obj.Attrs["origin"], ","); got != "AS65536" {
		t.Fatalf("origin attrs = %q, want AS65536", got)
	}
	if got := strings.Join(obj.Attrs["mnt-by"], ","); got != "FIRST-MNT,SECOND-MNT" {
		t.Fatalf("mnt-by attrs = %q", got)
	}
}

func TestParseContinuationLines(t *testing.T) {
	input := strings.NewReader(`route:          194.113.52.0/23
origin:         AS702
member-of:      AS702:RS-DE,
                AS702:RS-DE-PI
mnt-by:         WCOM-EMEA-RICE-MNT
source:         RIPE
`)

	objects, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if len(objects) != 1 {
		t.Fatalf("object count = %d, want 1", len(objects))
	}
	if got := strings.Join(objects[0].Attrs["member-of"], ","); got != "AS702:RS-DE, AS702:RS-DE-PI" {
		t.Fatalf("member-of attrs = %q", got)
	}
}

func TestParseInetnumExtractsStartAndEnd(t *testing.T) {
	input := strings.NewReader(`inetnum:        213.159.160.0 - 213.159.191.255
netname:        EXAMPLE-NET
mnt-by:         RIPE-NCC-HM-MNT
mnt-by:         ERICSSON-MNT
source:         RIPE
`)

	objects, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if len(objects) != 1 {
		t.Fatalf("object count = %d, want 1", len(objects))
	}
	obj := objects[0]
	if obj.Type != "inetnum" {
		t.Fatalf("Type = %q, want inetnum", obj.Type)
	}
	if obj.StartIP != "213.159.160.0" {
		t.Fatalf("StartIP = %q, want 213.159.160.0", obj.StartIP)
	}
	if obj.EndIP != "213.159.191.255" {
		t.Fatalf("EndIP = %q, want 213.159.191.255", obj.EndIP)
	}
	if got := strings.Join(obj.Attrs["mnt-by"], ","); got != "RIPE-NCC-HM-MNT,ERICSSON-MNT" {
		t.Fatalf("mnt-by attrs = %q", got)
	}
}

func TestParseCommentsAndInvalidUTF8(t *testing.T) {
	input := strings.NewReader("% header comment\n# local comment\nroute: 203.0.113.0/24\ndescr: S\xe3o Paulo\norigin: AS64496\nmnt-by: EXAMPLE-MNT\nsource: TEST\n")

	objects, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if len(objects) != 1 {
		t.Fatalf("object count = %d, want 1", len(objects))
	}
	obj := objects[0]
	if obj.Prefix != "203.0.113.0/24" {
		t.Fatalf("Prefix = %q, want 203.0.113.0/24", obj.Prefix)
	}
	if got := obj.Attrs["descr"][0]; !strings.Contains(got, "S") {
		t.Fatalf("descr attr = %q, want replacement-safe text", got)
	}
	if _, ok := obj.Attrs["% header comment"]; ok {
		t.Fatal("comment line was parsed as an attribute")
	}
}
