#!/bin/bash


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

#get_routes 'AS63179' >> /tmp/akamai.txt || echo 'failed'

python utils/arin-org.py AKAMAI >> /tmp/akamai.txt


# save ipv4
grep -v ':' /tmp/akamai.txt > /tmp/akamai-ipv4.txt

# save ipv6
grep ':' /tmp/akamai.txt > /tmp/akamai-ipv6.txt


# sort & uniq
sort -h /tmp/akamai-ipv4.txt | uniq > akamai/ipv4.txt
sort -h /tmp/akamai-ipv6.txt | uniq > akamai/ipv6.txt
