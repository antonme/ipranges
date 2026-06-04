#!/bin/bash

# https://www.bing.com/webmasters/help/verify-bingbot-2195837f

set -euo pipefail
set -x


# get from public ranges
curl -s https://www.bing.com/toolbox/bingbot.json > /tmp/bing_original.json


# Start from 2 Nov 2022 this list contains hidden character (\u200b) in some prefixes: https://i.imgur.com/I4LiPYr.png
# With this we remove all unprintable characters:
tr -cd "[:print:]\n" < /tmp/bing_original.json > /tmp/bing.json


# save ipv4
jq '.prefixes[] | [.ipv4Prefix][] | select(. != null)' -r /tmp/bing.json > /tmp/bing-ipv4.txt
curl https://raw.githubusercontent.com/antonme/ipnames/master/resolve-bing.txt >> /tmp/bing-ipv4.txt || echo 'failed'
#curl https://raw.githubusercontent.com/antonme/ipnames/master/ext-resolve-bing.txt >> /tmp/bing-ipv4.txt || echo 'failed'
# ipv6 not provided


# sort & uniq
sort -h /tmp/bing-ipv4.txt | uniq > bing/ipv4.txt
