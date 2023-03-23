#!/bin/bash

set -euo pipefail
set -x


# get from Autonomous System
get_maintained() {
    whois -h whois.ripe.net -- "-i mnt-by $1" | grep '^route' | awk '{ print $2; }'
    whois -h whois.radb.net -- "-i mnt-by $1" | grep '^route' | awk '{ print $2; }'
    whois -h rr.ntt.net -- "-i mnt-by $1" | grep '^route' | awk '{ print $2; }'
    sleep 5
    whois -h whois.ripe.net -- "-i mnt-by $1" | grep inetnum|awk '{$1=""; system("ipcalc -b -r "$2" "$4"| tail +2");}'
}




#get_maintained 'MNT-CLOUDMTS' > /tmp/mts.txt || echo 'failed'
get_maintained 'MTU-NOC' > /tmp/mts.txt || echo 'failed'
get_maintained 'MGTS-MNT' > /tmp/mts.txt || echo 'failed'
get_maintained 'MGTS-USPD-MNT' > /tmp/mts.txt || echo 'failed'


# save ipv4
grep -v ':' /tmp/mts.txt > /tmp/mts-ipv4.txt


# sort & uniq
sort -h /tmp/mts-ipv4.txt | uniq > mts/ipv4.txt
