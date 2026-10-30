package rpki

import (
	"reflect"
	"strings"
	"testing"

	"github.com/antonme/ipranges/irrindex/internal/rpsl"
)

func TestParseCSVConvertsVRPsToRouteObjects(t *testing.T) {
	input := `ASN,IP Prefix,Max Length,Trust Anchor,Expires
AS12389,83.166.227.0/24,24,ripe,1782742655
AS12389,2a00:1640::/32,48,ripe,1782742655
`

	got, err := ParseCSV(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCSV returned error: %v", err)
	}

	want := []rpsl.Object{
		{
			Type:   "route",
			Prefix: "83.166.227.0/24",
			Attrs: map[string][]string{
				"origin":       {"AS12389"},
				"max-length":   {"24"},
				"trust-anchor": {"ripe"},
				"expires":      {"1782742655"},
			},
		},
		{
			Type:   "route6",
			Prefix: "2a00:1640::/32",
			Attrs: map[string][]string{
				"origin":       {"AS12389"},
				"max-length":   {"48"},
				"trust-anchor": {"ripe"},
				"expires":      {"1782742655"},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("objects = %#v, want %#v", got, want)
	}
}
