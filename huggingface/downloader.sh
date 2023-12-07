#!/bin/bash


set -euo pipefail
set -x


# get from Autonomous System
get_routes() {
    whois -h riswhois.ripe.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }'
    whois -h whois.radb.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }'
    whois -h rr.ntt.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }'
    whois -h whois.rogerstelecom.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }'
    whois -h whois.bgp.net.br -- "-i origin $1" | rg '^route' | awk '{ print $2; }'
}

curl https://raw.githubusercontent.com/antonme/ipnames/master/resolve-huggingface.txt > /tmp/huggingface.txt || echo 'failed'
curl https://raw.githubusercontent.com/antonme/ipnames/master/ext-resolve-huggingface.txt >> /tmp/huggingface.txt || echo 'failed'


# save ipv4
grep -v ':' /tmp/huggingface.txt > /tmp/huggingface-ipv4.txt

# save ipv6
grep ':' /tmp/huggingface.txt > /tmp/huggingface-ipv6.txt

# sort & uniq
sort -h /tmp/huggingface-ipv4.txt | uniq > huggingface/ipv4.txt
sort -h /tmp/huggingface-ipv6.txt | uniq > huggingface/ipv6.txt
