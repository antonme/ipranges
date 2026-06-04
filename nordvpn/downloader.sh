#!/bin/bash


# https://www.irr.net/docs/list.html
# https://bgp.he.net/search?search%5Bsearch%5D=nordvpn&commit=Search
# https://github.com/SecOps-Institute/TwitterIPLists/blob/master/nordvpn_asn_list.lst

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

get_routes 'AS136787' > /tmp/nordvpn.txt || echo 'failed'

curl --silent "https://api.nordvpn.com/v1/servers?limit=16383"| jq .[].ips[].ip.ip|tr -d '"'| sort -h|uniq >>/tmp/nordvpn.txt


# save ipv4
grep -v ':' /tmp/nordvpn.txt > /tmp/nordvpn-ipv4.txt

# save ipv6
grep ':' /tmp/nordvpn.txt > /tmp/nordvpn-ipv6.txt


# sort & uniq
sort -h /tmp/nordvpn-ipv4.txt | uniq > nordvpn/ipv4.txt
sort -h /tmp/nordvpn-ipv6.txt | uniq > nordvpn/ipv6.txt
