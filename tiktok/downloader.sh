#!/bin/bash


# https://www.irr.net/docs/list.html
# https://bgp.he.net/search?search%5Bsearch%5D=tiktok&commit=Search
# https://github.com/SecOps-Institute/TwitterIPLists/blob/master/tiktok_asn_list.lst

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

get_routes 'AS138699' > /tmp/tiktok.txt || echo 'failed'

python utils/arin-org.py BYTED >> /tmp/tiktok.txt
curl https://raw.githubusercontent.com/antonme/ipnames/master/resolve-tiktok.txt >> /tmp/tiktok.txt || echo 'failed'
curl https://raw.githubusercontent.com/antonme/ipnames/master/ext-resolve-tiktok.txt >> /tmp/tiktok.txt || echo 'failed'

# save ipv4
grep -v ':' /tmp/tiktok.txt > /tmp/tiktok-ipv4.txt

# save ipv6
grep ':' /tmp/tiktok.txt > /tmp/tiktok-ipv6.txt


# sort & uniq
sort -h /tmp/tiktok-ipv4.txt | uniq > tiktok/ipv4.txt
sort -h /tmp/tiktok-ipv6.txt | uniq > tiktok/ipv6.txt
