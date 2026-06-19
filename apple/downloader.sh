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

get_maintained 'APPLE-MNT' | grep "/" > /tmp/apple.txt || echo "failed"
python utils/arin-org.py APPLEC-1-Z >> /tmp/apple.txt

# APPLEC-1-Z + APPLE-MNT give Apple's v4 (incl. 17.0.0.0/8) but no IPv6; Apple's
# announced IPv6 (~hundreds of route6 prefixes) lives on its origin ASNs.
get_routes 'AS714' >> /tmp/apple.txt || echo "failed"
get_routes 'AS6185' >> /tmp/apple.txt || echo "failed"

# drop any default route an origin query might surface; apple/ipv6 is first-populated
# here, so CI's sanity floor has no prior baseline to catch a stray /0.
grep -vE '^(0\.0\.0\.0|::)/0$' /tmp/apple.txt > /tmp/apple-clean.txt || echo "failed"

# save ipv4
grep -v ':' /tmp/apple-clean.txt > /tmp/apple-ipv4.txt || echo "failed"
sort -h /tmp/apple-ipv4.txt | uniq > apple/ipv4.txt || echo "failed"

# save ipv6
grep ':' /tmp/apple-clean.txt > /tmp/apple-ipv6.txt || echo "failed"
sort -h /tmp/apple-ipv6.txt | uniq > apple/ipv6.txt || echo "failed"
