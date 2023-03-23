#!/bin/bash


set -euo pipefail
set -x

curl -s https://www.cloudflare.com/ips-v4 > cloudflare/ipv4.txt
curl -s https://www.cloudflare.com/ips-v6 > cloudflare/ipv6.txt

echo >> cloudflare/ipv4.txt
echo >> cloudflare/ipv6.txt
