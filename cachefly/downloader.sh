#!/bin/bash

set -euo pipefail
set -x


get_maintained() {
    whois -h whois.ripe.net -- "-i mnt-by $1" > /tmp/ripe.txt
    cat /tmp/ripe.txt | rg '^route' | awk '{ print $2; }'
    whois -h whois.radb.net -- "-i mnt-by $1" | rg '^route' | awk '{ print $2; }'
    whois -h rr.ntt.net -- "-i mnt-by $1" | rg '^route' | awk '{ print $2; }'
	rg inetnum /tmp/ripe.txt |sort -h|uniq|awk '{print $2" "$4}'|python utils/ipcalc.py
}

#get_maintained 'YAHOO-MNT' | grep "/" > /tmp/cachefly.txt || echo "failed"

python utils/arin-org.py CL-1923 >> /tmp/cachefly.txt

# save ipv4
grep -v ':' /tmp/cachefly.txt > /tmp/cachefly-ipv4.txt || echo "failed"
sort -h /tmp/cachefly-ipv4.txt | uniq > cachefly/ipv4.txt || echo "failed"

# save ipv6
grep ':' /tmp/cachefly.txt > /tmp/cachefly-ipv6.txt || echo "failed"
sort -h /tmp/cachefly-ipv6.txt | uniq > cachefly/ipv6.txt || echo "failed"
