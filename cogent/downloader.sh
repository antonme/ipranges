#!/bin/bash


set -euo pipefail
set -x


# get from Autonomous System. Each IRR is queried independently with `|| true` so a
# server that is down — or simply has no route object for a small ASN — neither
# aborts the function (under `set -e`/`pipefail`) nor hides the other registries.
get_routes() {
    whois -h riswhois.ripe.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }' || true
    whois -h whois.radb.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }' || true
    whois -h rr.ntt.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }' || true
    whois -h whois.rogerstelecom.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }' || true
    whois -h whois.bgp.net.br -- "-i origin $1" | rg '^route' | awk '{ print $2; }' || true
}


# Cogent publishes an RFC 8805 geofeed, but it only lists geolocated sub-blocks:
# ~a third of Cogent's routed IPv4 space and almost none of its IPv6 aggregates.
# Union it with BGP route objects to get the full announced footprint. The geofeed
# is non-fatal: the AS174 route objects below are a strong fallback on their own.
curl -fsS --retry 3 --retry-delay 5 --retry-all-errors https://geofeed.cogentco.com/geofeed.csv | cut -d',' -f1 > /tmp/cogent.txt || echo 'geofeed failed'

# AS174 is ~all of Cogent's footprint. A handful of secondary Cogent Communications
# ASNs announce small extra blocks. AS1239 (legacy SprintLink, acquired from
# T-Mobile/Sprint in 2023) is currently dormant — its space has migrated into
# AS174 — but we still query it so any re-announced legacy Sprint space is caught.
for asn in AS174 AS1239 AS1790 AS1803 AS2149 AS4991; do
    get_routes "$asn" >> /tmp/cogent.txt || echo "failed: $asn"
done

# drop default routes that RIS reports for a Tier-1 origin; a /0 would otherwise
# swallow the entire merged list.
grep -vE '^(0\.0\.0\.0|::)/0$' /tmp/cogent.txt > /tmp/cogent-clean.txt

# save ipv4
grep -v ':' /tmp/cogent-clean.txt > /tmp/cogent-ipv4.txt

# save ipv6
grep ':' /tmp/cogent-clean.txt > /tmp/cogent-ipv6.txt


# sort & uniq
sort -h /tmp/cogent-ipv4.txt | uniq > cogent/ipv4.txt
sort -h /tmp/cogent-ipv6.txt | uniq > cogent/ipv6.txt
