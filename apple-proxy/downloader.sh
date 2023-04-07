#!/bin/bash
set -euo pipefail
set -x


curl -s https://mask-api.icloud.com/egress-ip-ranges.csv | cut -d',' -f1  > /tmp/apple-proxy.txt

cat /tmp/apple-proxy.txt | grep -v ":" > /tmp/apple-proxy-ipv4.txt
cat /tmp/apple-proxy.txt | grep ":" > /tmp/apple-proxy-ipv6.txt

sort -h /tmp/apple-proxy-ipv4.txt | uniq > apple-proxy/ipv4.txt
sort -h /tmp/apple-proxy-ipv6.txt | uniq > apple-proxy/ipv6.txt

