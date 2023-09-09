#!/bin/bash

set -euo pipefail
set -x


# get from Autonomous System
get_routes() {
    whois -h riswhois.ripe.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }'
    whois -h whois.radb.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }'
    whois -h rr.ntt.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }'
    whois -h whois.rogerstelecom.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }'
    whois -h whois.bgp.net.br -- "-i origin $1" | rg '^route' | awk '{ print $2; }'
}

get_routes 'AS19551' > /tmp/imperva.txt || echo 'failed'


# save ipv4
grep -v ':' /tmp/imperva.txt > /tmp/imperva-ipv4.txt

# save ipv6
grep ':' /tmp/imperva.txt > /tmp/imperva-ipv6.txt


# sort & uniq
sort -h /tmp/imperva-ipv4.txt | uniq > imperva/ipv4.txt
sort -h /tmp/imperva-ipv6.txt | uniq > imperva/ipv6.txt
