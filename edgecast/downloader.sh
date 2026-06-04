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

get_routes 'AS14153' > /tmp/edgecast.txt || echo 'failed'
get_routes 'AS15133' >> /tmp/edgecast.txt || echo 'failed'
get_routes 'AS14210' >> /tmp/edgecast.txt || echo 'failed'
python utils/arin-org.py OH-207 >> /tmp/edgecast.txt
python utils/arin-org.py EDGEC-25 >> /tmp/edgecast.txt

# save ipv4
grep -v ':' /tmp/edgecast.txt > /tmp/edgecast-ipv4.txt

# save ipv6
grep ':' /tmp/edgecast.txt > /tmp/edgecast-ipv6.txt


# sort & uniq
sort -h /tmp/edgecast-ipv4.txt | uniq > edgecast/ipv4.txt
sort -h /tmp/edgecast-ipv6.txt | uniq > edgecast/ipv6.txt
