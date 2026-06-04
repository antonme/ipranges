#!/bin/bash


# https://www.irr.net/docs/list.html
# https://bgp.he.net/search?search%5Bsearch%5D=protonvpn&commit=Search
# https://github.com/SecOps-Institute/TwitterIPLists/blob/master/protonvpn_asn_list.lst

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

get_routes 'AS209103' > /tmp/protonvpn.txt || echo 'failed'

curl --silent https://api.protonvpn.ch/vpn/logicals|jq .LogicalServers[].Servers[].EntryIP| tr -d '"'| sort -h|uniq >>/tmp/protonvpn.txt


# save ipv4
grep -v ':' /tmp/protonvpn.txt > /tmp/protonvpn-ipv4.txt

# save ipv6
grep ':' /tmp/protonvpn.txt > /tmp/protonvpn-ipv6.txt


# sort & uniq
sort -h /tmp/protonvpn-ipv4.txt | uniq > protonvpn/ipv4.txt
sort -h /tmp/protonvpn-ipv6.txt | uniq > protonvpn/ipv6.txt
