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


python utils/arin-org.py ZEITI >> /tmp/vercel.txt
#curl -s https://api.vercel.com/public-ip-list| jq -r '.addresses[], .ipv6_addresses[]' >> /tmp/vercel.txt

# save ipv4
grep -v ':' /tmp/vercel.txt > /tmp/vercel-ipv4.txt


# sort & uniq
sort -h /tmp/vercel-ipv4.txt | uniq > vercel/ipv4.txt
