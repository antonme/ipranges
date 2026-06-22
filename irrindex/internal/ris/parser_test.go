package ris

import (
	"os"
	"strings"
	"testing"
)

func TestParseSkipsCommentsAndNormalizesASN(t *testing.T) {
	file, err := os.Open("../../testdata/ris/riswhoisdump-ipv4.txt")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer file.Close()

	records, err := Parse(file)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("record count = %d, want 3", len(records))
	}
	if got := records[0]; got != (Record{ASN: "AS64073", Prefix: "0.0.0.0/0", PeerCount: 1, IsDefault: true}) {
		t.Fatalf("first record = %+v", got)
	}
	if got := records[1]; got != (Record{ASN: "AS16625", Prefix: "23.32.0.0/11", PeerCount: 12}) {
		t.Fatalf("second record = %+v", got)
	}
	if got := records[2]; got != (Record{ASN: "AS714", Prefix: "17.0.0.0/8", PeerCount: 25}) {
		t.Fatalf("third record = %+v", got)
	}
}

func TestParseHandlesPrefixedASNAndIPv6Default(t *testing.T) {
	input := strings.NewReader("AS64496 2001:db8::/32 4\n64497 ::/0 2\n")

	records, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if got := records[0]; got != (Record{ASN: "AS64496", Prefix: "2001:db8::/32", PeerCount: 4}) {
		t.Fatalf("first record = %+v", got)
	}
	if got := records[1]; got != (Record{ASN: "AS64497", Prefix: "::/0", PeerCount: 2, IsDefault: true}) {
		t.Fatalf("second record = %+v", got)
	}
}
