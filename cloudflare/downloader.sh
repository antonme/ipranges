#!/bin/bash


set -euo pipefail
set -x

# Stage in /tmp first so a failed fetch can't truncate the committed list.
curl -fsS --retry 3 --retry-delay 5 --retry-all-errors https://www.cloudflare.com/ips-v4 > /tmp/cloudflare-ipv4.txt
curl -fsS --retry 3 --retry-delay 5 --retry-all-errors https://www.cloudflare.com/ips-v6 > /tmp/cloudflare-ipv6.txt

echo >> /tmp/cloudflare-ipv4.txt
echo >> /tmp/cloudflare-ipv6.txt

mv /tmp/cloudflare-ipv4.txt cloudflare/ipv4.txt
mv /tmp/cloudflare-ipv6.txt cloudflare/ipv6.txt
