#!/bin/bash

# https://www.workplace.com/resources/tech/it-configuration/domain-whitelisting
# https://www.irr.net/docs/list.html
# https://bgp.he.net/search?search%5Bsearch%5D=facebook&commit=Search
# https://github.com/SecOps-Institute/FacebookIPLists/blob/master/facebook_asn_list.lst

set -euo pipefail
set -x


# get from Autonomous System. Each IRR is queried independently with `|| true` so a
# server that is down — or simply has no route object for the ASN — neither aborts the
# function (under `set -e`/`pipefail`) nor hides the other registries.
get_routes() {
    whois -h riswhois.ripe.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }' || true
    whois -h whois.radb.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }' || true
    whois -h rr.ntt.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }' || true
    whois -h whois.rogerstelecom.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }' || true
    whois -h whois.bgp.net.br -- "-i origin $1" | rg '^route' | awk '{ print $2; }' || true
}

python utils/arin-org.py AL-3 > /tmp/alibaba.txt
whois -h whois.apnic.net "TAOBAO"| rg inetnum|sort -h|uniq|awk '{print $2" "$4}'|python utils/ipcalc.py >>/tmp/alibaba.txt
whois -h whois.apnic.net "TAOBAO"| grep inet6num| awk '{print $2}' >> /tmp/alibaba.txt

# The ARIN AL-3 org + APNIC TAOBAO handle above cover only part of Alibaba. Add the
# announced Alibaba Cloud / Aliyun footprint registered elsewhere: AS37963 (mainland
# Aliyun, the biggest block), the non-AL-3 half of AS45102 (Alibaba US), Taobao's tail
# (AS24429), and the Singapore cloud ASNs (AS134963, AS203513).
for asn in AS37963 AS45102 AS24429 AS134963 AS203513; do
    get_routes "$asn" >> /tmp/alibaba.txt || echo "failed: $asn"
done

# drop any default route an origin query might surface; a /0 would swallow the merged list.
grep -vE '^(0\.0\.0\.0|::)/0$' /tmp/alibaba.txt > /tmp/alibaba-clean.txt

# save ipv4
grep -v ':' /tmp/alibaba-clean.txt > /tmp/alibaba-ipv4.txt

# save ipv6
grep ':' /tmp/alibaba-clean.txt > /tmp/alibaba-ipv6.txt


# sort & uniq
sort -h /tmp/alibaba-ipv4.txt | uniq > alibaba/ipv4.txt
sort -h /tmp/alibaba-ipv6.txt | uniq > alibaba/ipv6.txt
