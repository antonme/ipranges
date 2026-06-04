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


#python utils/arin-org.py ZEITI >> /tmp/constant.txt
curl -fsS --retry 3 --retry-delay 5 --retry-all-errors https://geofeed.constant.com/ > /tmp/constant.txt

# save ipv4
grep -v ':' /tmp/constant.txt > /tmp/constant-ipv4.txt

# save ipv6
grep ':' /tmp/constant.txt > /tmp/constant-ipv6.txt


# sort & uniq
sort -h /tmp/constant-ipv4.txt | uniq > constant/ipv4.txt
sort -h /tmp/constant-ipv6.txt | uniq > constant/ipv6.txt
