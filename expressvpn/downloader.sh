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

# LogicWeb
get_routes 'AS64286' > /tmp/expressvpn.txt || echo 'failed'
# IPXO Limited
get_routes 'AS206092' >> /tmp/expressvpn.txt || echo 'failed'
# Latitude.sh
get_routes 'AS262287' >> /tmp/expressvpn.txt || echo 'failed'
# Angani Limited
get_routes 'AS37684' >> /tmp/expressvpn.txt || echo 'failed'


curl https://raw.githubusercontent.com/antonme/ipnames/master/resolve-expressvpn.txt >> /tmp/expressvpn.txt || echo 'failed'


# save ipv4
grep -v ':' /tmp/expressvpn.txt > /tmp/expressvpn-ipv4.txt

# save ipv6
grep ':' /tmp/expressvpn.txt > /tmp/expressvpn-ipv6.txt


# sort & uniq
sort -h /tmp/expressvpn-ipv4.txt | uniq > expressvpn/ipv4.txt
sort -h /tmp/expressvpn-ipv6.txt | uniq > expressvpn/ipv6.txt
