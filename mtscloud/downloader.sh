#!/bin/bash

set -euo pipefail
set -x


# get from Autonomous System
get_maintained() {
    whois -h whois.ripe.net -- "-i mnt-by $1" > /tmp/ripe.txt
    cat /tmp/ripe.txt | rg '^route' | awk '{ print $2; }'
    whois -h whois.radb.net -- "-i mnt-by $1" | rg '^route' | awk '{ print $2; }'
    whois -h rr.ntt.net -- "-i mnt-by $1" | rg '^route' | awk '{ print $2; }'
    rg inetnum /tmp/ripe.txt |sort -h|uniq|awk '{print $2" "$4}'|python utils/ipcalc.py
}




get_maintained 'MNT-CLOUDMTS' > /tmp/mtscloud.txt || echo 'failed'


# save ipv4
grep -v ':' /tmp/mtscloud.txt > /tmp/mtscloud-ipv4.txt


# sort & uniq
sort -h /tmp/mtscloud-ipv4.txt | uniq > mtscloud/ipv4.txt
