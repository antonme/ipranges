#!/bin/bash

set -euo pipefail
set -x


# get from Autonomous System
get_maintained() {
    whois -h whois.ripe.net -- "-i mnt-by $1" | rg '^route' | awk '{ print $2; }'
    whois -h whois.radb.net -- "-i mnt-by $1" | rg '^route' | awk '{ print $2; }'
    whois -h rr.ntt.net -- "-i mnt-by $1" | rg '^route' | awk '{ print $2; }'
    whois -h whois.rogerstelecom.net -- "-i mnt-by $1" | rg '^route' | awk '{ print $2; }'
    whois -h whois.bgp.net.br -- "-i mnt-by $1" | rg '^route' | awk '{ print $2; }'
}

get_maintained 'OZONRU-MNT' > /tmp/ozonru.txt || echo 'failed'


nslookup ozon.ru| tail -n +4 | grep Address| awk '{print $2}' >> /tmp/ozonru.txt

# save ipv4
grep -v ':' /tmp/ozonru.txt > /tmp/ozonru-ipv4.txt

# save ipv6
grep ':' /tmp/ozonru.txt > /tmp/ozonru-ipv6.txt


# sort & uniq
sort -h /tmp/ozonru-ipv4.txt | uniq > ozonru/ipv4.txt
sort -h /tmp/ozonru-ipv6.txt | uniq > ozonru/ipv6.txt
