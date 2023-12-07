#!/bin/bash


# https://www.irr.net/docs/list.html
# https://bgp.he.net/search?search%5Bsearch%5D=backblaze&commit=Search
# https://github.com/SecOps-Institute/TwitterIPLists/blob/master/backblaze_asn_list.lst

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

get_routes 'AS40401' > /tmp/backblaze.txt || echo 'failed'

python utils/arin-org.py AS40401 >> /tmp/backblaze.txt


curl https://raw.githubusercontent.com/antonme/ipnames/master/resolve-backblaze.txt >> /tmp/backblaze.txt || echo 'failed'
curl https://raw.githubusercontent.com/antonme/ipnames/master/ext-resolve-backblaze.txt >> /tmp/backblaze.txt || echo 'failed'


# save ipv4
grep -v ':' /tmp/backblaze.txt > /tmp/backblaze-ipv4.txt

# save ipv6
grep ':' /tmp/backblaze.txt > /tmp/backblaze-ipv6.txt



# sort & uniq
sort -h /tmp/backblaze-ipv4.txt | uniq > backblaze/ipv4.txt
sort -h /tmp/backblaze-ipv6.txt | uniq > backblaze/ipv6.txt
