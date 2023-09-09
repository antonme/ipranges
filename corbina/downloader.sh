#!/bin/bash

set -euo pipefail
set -x


# get from Autonomous System
get_maintained() {
    whois -h whois.ripe.net -- "-i mnt-by $1" > /tmp/ripe.txt
    cat /tmp/ripe.txt | grep '^route' | awk '{ print $2; }'
    whois -h whois.radb.net -- "-i mnt-by $1" | grep '^route' | awk '{ print $2; }'
    whois -h rr.ntt.net -- "-i mnt-by $1" | grep '^route' | awk '{ print $2; }'
	rg inetnum /tmp/ripe.txt |sort -h|uniq|awk '{print $2" "$4}'|python utils/ipcalc.py
}




#get_maintained 'MNT-CLOUDMTS' > /tmp/corbina.txt || echo 'failed'
get_maintained 'RU-CORBINA-MNT' > /tmp/corbina.txt || echo 'failed'


# save ipv4
grep -v ':' /tmp/corbina.txt | grep "/" > /tmp/corbina-ipv4.txt


# sort & uniq
sort -h /tmp/corbina-ipv4.txt | uniq > corbina/ipv4.txt
