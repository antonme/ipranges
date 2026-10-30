package mirrors

import "testing"

func TestSourcesForHostMatchesMirroredDatabases(t *testing.T) {
	tests := []struct {
		host    string
		want    []string
		notWant []string
	}{
		{
			host:    "whois.ripe.net",
			want:    []string{"ripe_route", "ripe_nonauth_route", "ripe_nonauth_route6", "ripe_inetnum"},
			notWant: []string{"radb", "rpki"},
		},
		{
			host:    "whois.radb.net",
			want:    []string{"radb", "ripe_nonauth_route", "ripe_nonauth_route6", "altdb", "reach", "afrinic", "bgp_net_br"},
			notWant: []string{"rpki", "ripe_inetnum", "ris"},
		},
		{
			host:    "rr.ntt.net",
			want:    []string{"nttcom", "radb", "arin_irr", "ripe_route", "ripe_nonauth_route", "apnic_route6", "bgp_net_br", "altdb", "afrinic", "rpki"},
			notWant: []string{"reach", "ripe_inetnum", "ris"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.host, func(t *testing.T) {
			got, ok := SourcesForHost(tc.host)
			if !ok {
				t.Fatalf("SourcesForHost(%q) ok = false", tc.host)
			}
			for _, want := range tc.want {
				if !contains(got, want) {
					t.Errorf("sources = %#v, want %s", got, want)
				}
			}
			for _, notWant := range tc.notWant {
				if contains(got, notWant) {
					t.Errorf("sources = %#v, must not include %s", got, notWant)
				}
			}
		})
	}
}

func TestSourcesForHostAgreesWithSources(t *testing.T) {
	for _, host := range []string{"riswhois.ripe.net", "whois.ripe.net", "whois.radb.net", "rr.ntt.net", "whois.rogerstelecom.net", "whois.bgp.net.br"} {
		byHost, ok := SourcesForHost(host)
		if !ok {
			t.Fatalf("SourcesForHost(%q) ok = false", host)
		}
		byMirror, _ := Sources(Name(host))
		if len(byHost) != len(byMirror) {
			t.Fatalf("%s: host sources %#v != mirror sources %#v", host, byHost, byMirror)
		}
	}
}

func TestSourcesUnknownMirror(t *testing.T) {
	if _, ok := SourcesForHost("whois.example.net"); ok {
		t.Fatal("SourcesForHost ok = true for unknown host")
	}
	if got := Name("WHOIS.RADB.NET"); got != "radb" {
		t.Fatalf("Name = %q, want radb", got)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
