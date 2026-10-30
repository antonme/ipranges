// Package mirrors maps the WHOIS servers queried by downloader scripts to the
// local index sources that emulate their answers.
package mirrors

import "strings"

// sourcesByMirror lists the local sources each WHOIS server answers from.
// Aggregate IRRd mirrors follow each server's `-q sources` output (checked
// 2026-10-30); only sources that have a local dump are listed. IRR mirrors
// carry route objects, not inetnum, so only RIPE itself maps to RIPE inetnums.
var sourcesByMirror = map[string][]string{
	"ris": {"ris", "ris_ipv4", "ris_ipv6"},
	// RIPE answers from RIPE and RIPE-NONAUTH by default.
	"ripe": {"ripe", "ripe_route", "ripe_route6", "ripe_nonauth_route", "ripe_nonauth_route6", "ripe_inetnum", "ripe_inet6num"},
	// RADb mirrors RIPE, RIPE-NONAUTH, ARIN, APNIC, AFRINIC, NTTCOM, ALTDB,
	// REACH and TC, among others without public dumps (LEVEL3, JPIRR, ...).
	"radb": {
		"radb",
		"arin_irr",
		"ripe_route",
		"ripe_route6",
		"ripe_nonauth_route",
		"ripe_nonauth_route6",
		"apnic_route",
		"apnic_route6",
		"nttcom",
		"bgp_net_br",
		"altdb",
		"reach",
		"afrinic",
	},
	// NTT mirrors the same set as RADb except REACH, and also serves IRRd's
	// RPKI pseudo-source, which `-q sources` does not list.
	"ntt": {
		"ntt",
		"nttcom",
		"nttcom_route",
		"nttcom_route6",
		"radb",
		"arin_irr",
		"ripe_route",
		"ripe_route6",
		"ripe_nonauth_route",
		"ripe_nonauth_route6",
		"apnic_route",
		"apnic_route6",
		"bgp_net_br",
		"altdb",
		"afrinic",
		"rpki",
	},
	"rogers":     {"rogers", "rogerstelecom"},
	"bgp_net_br": {"bgp_net_br", "bgp.net.br"},
}

// Name returns the mirror name for a WHOIS host, or the host itself when it is
// not a known mirror.
func Name(host string) string {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "riswhois.ripe.net":
		return "ris"
	case "whois.radb.net":
		return "radb"
	case "whois.ripe.net":
		return "ripe"
	case "rr.ntt.net":
		return "ntt"
	case "whois.rogerstelecom.net":
		return "rogers"
	case "whois.bgp.net.br":
		return "bgp_net_br"
	default:
		return host
	}
}

// Sources returns the local sources for a mirror name and whether the mirror
// is known.
func Sources(mirror string) ([]string, bool) {
	sources, ok := sourcesByMirror[mirror]
	if !ok {
		return nil, false
	}
	return append([]string(nil), sources...), true
}

// SourcesForHost returns the local sources for a WHOIS host and whether the
// host is known.
func SourcesForHost(host string) ([]string, bool) {
	return Sources(Name(host))
}
