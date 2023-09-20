#!/bin/bash

# https://www.workplace.com/resources/tech/it-configuration/domain-whitelisting
# https://www.irr.net/docs/list.html
# https://bgp.he.net/search?search%5Bsearch%5D=facebook&commit=Search
# https://github.com/SecOps-Institute/FacebookIPLists/blob/master/facebook_asn_list.lst

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

get_maintained 'MELBICOM-MNT' > /tmp/kinopub.txt || echo 'failed'


# save ipv4
grep -v ':' /tmp/kinopub.txt > /tmp/kinopub-ipv4.txt

# save ipv6
grep ':' /tmp/kinopub.txt > /tmp/kinopub-ipv6.txt


# sort & uniq
sort -h /tmp/kinopub-ipv4.txt | uniq > kinopub/ipv4.txt
sort -h /tmp/kinopub-ipv6.txt | uniq > kinopub/ipv6.txt
