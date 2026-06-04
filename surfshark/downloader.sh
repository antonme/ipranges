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

# Cyberzone S.A. 
get_routes 'AS209854' > /tmp/surfshark.txt || echo 'failed'


curl https://raw.githubusercontent.com/antonme/ipnames/master/resolve-surfshark.txt >> /tmp/surfshark.txt || echo 'failed'


# save ipv4
grep -v ':' /tmp/surfshark.txt > /tmp/surfshark-ipv4.txt

# save ipv6
grep ':' /tmp/surfshark.txt > /tmp/surfshark-ipv6.txt


# sort & uniq
sort -h /tmp/surfshark-ipv4.txt | uniq > surfshark/ipv4.txt
sort -h /tmp/surfshark-ipv6.txt | uniq > surfshark/ipv6.txt
