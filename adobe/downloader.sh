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

get_routes 'AS15224' > /tmp/adobe.txt || echo 'failed'
get_routes 'ADOBES-Z' >> /tmp/adobe.txt || echo 'failed'

python utils/arin-org.py AS15224 >> /tmp/adobe.txt


curl -f https://raw.githubusercontent.com/antonme/ipnames/master/resolve-adobe.txt >> /tmp/adobe.txt || echo 'failed'
curl -f https://raw.githubusercontent.com/antonme/ipnames/master/ext-resolve-adobe.txt >> /tmp/adobe.txt || echo 'failed'


# save ipv4
grep -v ':' /tmp/adobe.txt > /tmp/adobe-ipv4.txt

# save ipv6
grep ':' /tmp/adobe.txt > /tmp/adobe-ipv6.txt



# sort & uniq
sort -h /tmp/adobe-ipv4.txt | uniq > adobe/ipv4.txt
sort -h /tmp/adobe-ipv6.txt | uniq > adobe/ipv6.txt
