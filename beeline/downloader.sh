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




#get_maintained 'MNT-CLOUDMTS' > /tmp/beeline.txt || echo 'failed'
get_maintained 'BEE-MNT' > /tmp/beeline.txt || echo 'failed'


# save ipv4
grep -v ':' /tmp/beeline.txt > /tmp/beeline-ipv4.txt


# sort & uniq
sort -h /tmp/beeline-ipv4.txt | uniq > beeline/ipv4.txt
