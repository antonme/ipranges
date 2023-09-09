#!/bin/bash

set -euo pipefail
set -x


# get from Autonomous System
get_maintained() {
    whois -h whois.ripe.net -- "-i mnt-by $1" > /tmp/ripe.txt
    cat /tmp/ripe.txt | grep '^route' | awk '{ print $2; }'
    whois -h whois.radb.net -- "-i mnt-by $1" | grep '^route' | awk '{ print $2; }'
    whois -h rr.ntt.net -- "-i mnt-by $1" | grep '^route' | awk '{ print $2; }'
    rg inetnum /tmp/ripe.txt |awk '{$1=""; system("ipcalc -b -r "$2" "$4"| tail +2");}'
}




get_maintained 'MNT-CLOUDMTS' > /tmp/mtscloud.txt || echo 'failed'


# save ipv4
grep -v ':' /tmp/mtscloud.txt > /tmp/mtscloud-ipv4.txt


# sort & uniq
sort -h /tmp/mtscloud-ipv4.txt | uniq > mtscloud/ipv4.txt
