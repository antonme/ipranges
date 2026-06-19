#!/bin/bash


# https://www.hetzner.com/community/questions/19247/list-of-hetzners-ip-ranges

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

get_routes 'AS24940' > /tmp/hetzner.txt || echo 'failed'
# Hetzner Cloud announces from dedicated ASNs not registered under AS24940:
get_routes 'AS213230' >> /tmp/hetzner.txt || echo 'failed'
get_routes 'AS212317' >> /tmp/hetzner.txt || echo 'failed'

# save ipv4
grep -v ':' /tmp/hetzner.txt > /tmp/hetzner-ipv4.txt

# save ipv6
grep ':' /tmp/hetzner.txt > /tmp/hetzner-ipv6.txt


# sort & uniq
sort -h /tmp/hetzner-ipv4.txt | uniq > hetzner/ipv4.txt
sort -h /tmp/hetzner-ipv6.txt | uniq > hetzner/ipv6.txt
