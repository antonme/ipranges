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




get_maintained 'MNT-AVITO' > /tmp/avito.txt || echo 'failed'



# save ipv4
grep -v ':' /tmp/avito.txt > /tmp/avito-ipv4.txt

# save ipv6
grep ':' /tmp/avito.txt > /tmp/avito-ipv6.txt


# sort & uniq
sort -h /tmp/avito-ipv4.txt | uniq > avito/ipv4.txt
sort -h /tmp/avito-ipv6.txt | uniq > avito/ipv6.txt
