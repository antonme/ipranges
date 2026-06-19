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

# LinkedIn is a Microsoft subsidiary but announces its own ASNs and address space,
# distinct from the microsoft/ list. Only these three are actually routed; LinkedIn's
# other registered ASNs (AS20049, AS20366, LYNDA-COM, ...) announce nothing today.
get_routes 'AS14413' > /tmp/linkedin.txt || echo 'failed'   # largest (108.174.0.0/20, 144.2.x)
get_routes 'AS13443' >> /tmp/linkedin.txt || echo 'failed'  # main LinkedIn block
get_routes 'AS40793' >> /tmp/linkedin.txt || echo 'failed'  # 108.174.5-7.0/24

# save ipv4
grep -v ':' /tmp/linkedin.txt > /tmp/linkedin-ipv4.txt

# save ipv6
grep ':' /tmp/linkedin.txt > /tmp/linkedin-ipv6.txt


# sort & uniq
sort -h /tmp/linkedin-ipv4.txt | uniq > linkedin/ipv4.txt
sort -h /tmp/linkedin-ipv6.txt | uniq > linkedin/ipv6.txt
