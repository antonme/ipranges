#!/bin/bash


set -euo pipefail
set -x



# save ipv4
#grep -v ':' youtube/nets.txt > /tmp/youtube-ipv4.txt
curl -fsS --retry 3 --retry-delay 5 --retry-all-errors https://raw.githubusercontent.com/antonme/ipnames/master/resolve-youtube.txt > /tmp/youtube-ipv4.txt || echo 'failed'
curl -fsS --retry 3 --retry-delay 5 --retry-all-errors https://raw.githubusercontent.com/antonme/ipnames/master/ext-resolve-youtube.txt >> /tmp/youtube-ipv4.txt || echo 'failed'

# save ipv6
# grep ':' youtube/nets.txt > /tmp/youtube-ipv6.txt


# sort & uniq
sort -h /tmp/youtube-ipv4.txt | uniq > youtube/ipv4.txt
# sort -h /tmp/youtube-ipv6.txt | uniq > youtube/ipv6.txt
