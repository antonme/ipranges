#!/bin/bash

set -euo pipefail
set -x


# get from Autonomous System
get_maintained() {
    whois -h whois.ripe.net -- "-i mnt-by $1" >/tmp/ripe.txt
    cat /tmp/ripe.txt | rg '^route' | awk '{ print $2; }'
    whois -h whois.radb.net -- "-i mnt-by $1" | rg '^route' | awk '{ print $2; }'
    whois -h rr.ntt.net -- "-i mnt-by $1" | rg '^route' | awk '{ print $2; }'
	rg inetnum /tmp/ripe.txt |sort -h|uniq|awk '{print $2" "$4}'|python utils/ipcalc.py
}




#get_maintained 'MNT-CLOUDMTS' > /tmp/mts.txt || echo 'failed'
get_maintained 'MTU-NOC' > /tmp/mts.txt || echo 'failed'
sleep 10
get_maintained 'MGTS-MNT' >> /tmp/mts.txt || echo 'failed'
sleep 10
get_maintained 'MGTS-USPD-MNT' >> /tmp/mts.txt || echo 'failed'


# save ipv4
grep -v ':' /tmp/mts.txt > /tmp/mts-ipv4.txt

# save ipv6
grep ':' /tmp/mts.txt > /tmp/mts-ipv6.txt

# sort & uniq
sort -h /tmp/mts-ipv4.txt | uniq > mts/ipv4.txt
sort -h /tmp/mts-ipv6.txt | uniq > mts/ipv6.txt
