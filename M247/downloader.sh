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

get_routes 'AS9009' > /tmp/M247.txt || echo 'failed'

# save ipv4
grep -v ':' /tmp/M247.txt > /tmp/M247-ipv4.txt

# save ipv6
grep ':' /tmp/M247.txt > /tmp/M247-ipv6.txt


# sort & uniq
sort -h /tmp/M247-ipv4.txt | uniq > M247/ipv4.txt
sort -h /tmp/M247-ipv6.txt | uniq > M247/ipv6.txt
