#!/bin/bash

set -euo pipefail
set -x


get_maintained() {
    whois -h whois.ripe.net -- "-i mnt-by $1" > /tmp/ripe.txt
    cat /tmp/ripe.txt | rg '^route' | awk '{ print $2; }'
    whois -h whois.radb.net -- "-i mnt-by $1" | rg '^route' | awk '{ print $2; }'
    whois -h rr.ntt.net -- "-i mnt-by $1" | rg '^route' | awk '{ print $2; }'
	rg inetnum /tmp/ripe.txt |sort -h|uniq|awk '{print $2" "$4}'|python utils/ipcalc.py
}
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

get_maintained 'YAHOO-MNT' | grep "/" > /tmp/yahoo.txt || echo "failed"

# YAHOO-MNT is RIPE-only (EMEA). Yahoo's main US footprint announces under ARIN ASNs:
for asn in AS10310 AS26101 AS36646; do
    get_routes "$asn" >> /tmp/yahoo.txt || echo "failed: $asn"
done

# save ipv4
grep -v ':' /tmp/yahoo.txt > /tmp/yahoo-ipv4.txt || echo "failed"
sort -h /tmp/yahoo-ipv4.txt | uniq > yahoo/ipv4.txt || echo "failed"

# save ipv6
grep ':' /tmp/yahoo.txt > /tmp/yahoo-ipv6.txt || echo "failed"
sort -h /tmp/yahoo-ipv6.txt | uniq > yahoo/ipv6.txt || echo "failed"
