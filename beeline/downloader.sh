#!/bin/bash

set -euo pipefail
set -x


# get from Autonomous System
get_maintained() {
    whois -h whois.ripe.net -- "-i mnt-by $1" >/tmp/ripe.txt
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


#get_maintained 'MNT-CLOUDMTS' > /tmp/beeline.txt || echo 'failed'
get_maintained 'BEE-MNT' > /tmp/beeline.txt || echo 'failed'

# BEE-MNT alone covers very little of Beeline/VimpelCom. The core announces under these
# origins (its route objects are split across RU-CORBINA-MNT / AS3216-MNT / SOVINTEL-MNT
# maintainers, so a single-maintainer query misses most of it).
for asn in AS3216 AS16345; do
    get_routes "$asn" >> /tmp/beeline.txt || echo "failed: $asn"
done

# drop any default route an origin query might surface; a /0 would swallow the merged list.
grep -vE '^(0\.0\.0\.0|::)/0$' /tmp/beeline.txt > /tmp/beeline-clean.txt

# save ipv4
grep -v ':' /tmp/beeline-clean.txt > /tmp/beeline-ipv4.txt

# save ipv6
grep ':' /tmp/beeline-clean.txt > /tmp/beeline-ipv6.txt


# sort & uniq
sort -h /tmp/beeline-ipv4.txt | uniq > beeline/ipv4.txt
sort -h /tmp/beeline-ipv6.txt | uniq > beeline/ipv6.txt
