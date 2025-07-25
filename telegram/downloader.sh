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

get_maintained 'MNT-TELEGRAM' | grep -v 'route-set' | grep "/" > /tmp/telegram.txt || echo 'failed'


# save ipv4
grep -v ':' /tmp/telegram.txt > /tmp/telegram-ipv4.txt

# save ipv6
grep ':' /tmp/telegram.txt > /tmp/telegram-ipv6.txt


# sort & uniq
sort -h /tmp/telegram-ipv4.txt | uniq > telegram/ipv4.txt
sort -h /tmp/telegram-ipv6.txt | uniq > telegram/ipv6.txt
