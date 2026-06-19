#!/bin/bash

set -euo pipefail
set -x


# get from Autonomous System. Each IRR is queried independently with `|| true` so a
# server that is down — or simply has no route object for the ASN — neither aborts the
# function (under `set -e`/`pipefail`) nor hides the other registries.
get_routes() {
    whois -h riswhois.ripe.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }' || true
    whois -h whois.radb.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }' || true
    whois -h rr.ntt.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }' || true
    whois -h whois.rogerstelecom.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }' || true
    whois -h whois.bgp.net.br -- "-i origin $1" | rg '^route' | awk '{ print $2; }' || true
}

get_routes 'AS19551' > /tmp/imperva.txt || echo 'failed'
get_routes 'AS62571' >> /tmp/imperva.txt || echo 'failed'

# Imperva/Incapsula's official public egress feed (the addresses Incapsula connects to
# customer origins from). No auth required; documented POST endpoint.
curl -fsS --retry 3 --retry-delay 5 --retry-all-errors -X POST https://my.incapsula.com/api/integration/v1/ips | jq -r '.ipRanges[], .ipv6Ranges[]' >> /tmp/imperva.txt || echo 'failed'


# save ipv4
grep -v ':' /tmp/imperva.txt > /tmp/imperva-ipv4.txt

# save ipv6
grep ':' /tmp/imperva.txt > /tmp/imperva-ipv6.txt


# sort & uniq
sort -h /tmp/imperva-ipv4.txt | uniq > imperva/ipv4.txt
sort -h /tmp/imperva-ipv6.txt | uniq > imperva/ipv6.txt
