#!/bin/bash


# https://www.hetzner.com/community/questions/19247/list-of-hetzners-ip-ranges

set -euo pipefail
set -x
# get from Autonomous System
get_routes() {
    whois -h riswhois.ripe.net -- "-i origin $1" | grep '^route' | awk '{ print $2; }'
    whois -h whois.radb.net -- "-i origin $1" | grep '^route' | awk '{ print $2; }'
    whois -h rr.ntt.net -- "-i origin $1" | grep '^route' | awk '{ print $2; }'
    whois -h whois.rogerstelecom.net -- "-i origin $1" | grep '^route' | awk '{ print $2; }'
    whois -h whois.bgp.net.br -- "-i origin $1" | grep '^route' | awk '{ print $2; }'
}

get_routes 'AS24940' > /tmp/hetzner.txt || echo 'failed'

# save ipv4
grep -v ':' /tmp/hetzner.txt > /tmp/hetzner-ipv4.txt

# save ipv6
grep ':' /tmp/hetzner.txt > /tmp/hetzner-ipv6.txt


# sort & uniq
sort -h /tmp/hetzner-ipv4.txt | uniq > hetzner/ipv4.txt
sort -h /tmp/hetzner-ipv6.txt | uniq > hetzner/ipv6.txt
