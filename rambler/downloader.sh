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

get_maintained 'RAMBLER-MNT' > /tmp/rambler.txt || echo 'failed'


# save ipv4
grep -v ':' /tmp/rambler.txt > /tmp/rambler-ipv4.txt

# save ipv6
grep ':' /tmp/rambler.txt > /tmp/rambler-ipv6.txt


# sort & uniq
sort -h /tmp/rambler-ipv4.txt | uniq > rambler/ipv4.txt
sort -h /tmp/rambler-ipv6.txt | uniq > rambler/ipv6.txt
