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

get_maintained 'SBRF-mnt' > /tmp/sber.txt || echo 'failed'
get_maintained 'PVBSBRF-MNT-RIPE' >> /tmp/sber.txt || echo 'failed'


# save ipv4
grep -v ':' /tmp/sber.txt > /tmp/sber-ipv4.txt

# save ipv6
grep ':' /tmp/sber.txt > /tmp/sber-ipv6.txt


# sort & uniq
sort -h /tmp/sber-ipv4.txt | uniq > sber/ipv4.txt
sort -h /tmp/sber-ipv6.txt | uniq > sber/ipv6.txt
