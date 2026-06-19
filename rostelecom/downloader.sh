#!/bin/bash

set -euo pipefail
set -x


# get from Autonomous System
get_maintained() {
    whois -h whois.ripe.net -- "-i mnt-by $1" | rg '^route' | awk '{ print $2; }'
    whois -h whois.radb.net -- "-i mnt-by $1" | rg '^route' | awk '{ print $2; }'
    whois -h rr.ntt.net -- "-i mnt-by $1" | rg '^route' | awk '{ print $2; }'
    whois -h whois.rogerstelecom.net -- "-i mnt-by $1" | rg '^route' | awk '{ print $2; }'
    whois -h whois.bgp.net.br -- "-i mnt-by $1" | rg '^route' | awk '{ print $2; }'
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

get_maintained 'ROSTELECOM-MNT' | grep -v 'route-set' | grep "/" > /tmp/rostelecom.txt || echo 'failed'

# Rostelecom absorbed regional operators that announce under their OWN maintainers, so
# the single ROSTELECOM-MNT query above misses them: AS28840 (Tattelecom), AS35807
# (SkyNet SPb), AS42610 (NCNet) — ~3M+ extra addresses.
for asn in AS28840 AS35807 AS42610; do
    get_routes "$asn" >> /tmp/rostelecom.txt || echo "failed: $asn"
done

# drop any default route an origin query might surface; a /0 would swallow the merged list.
grep -vE '^(0\.0\.0\.0|::)/0$' /tmp/rostelecom.txt > /tmp/rostelecom-clean.txt

# save ipv4
grep -v ':' /tmp/rostelecom-clean.txt > /tmp/rostelecom-ipv4.txt

# save ipv6
grep ':' /tmp/rostelecom-clean.txt > /tmp/rostelecom-ipv6.txt


# sort & uniq
sort -h /tmp/rostelecom-ipv4.txt | uniq > rostelecom/ipv4.txt
sort -h /tmp/rostelecom-ipv6.txt | uniq > rostelecom/ipv6.txt
