#!/bin/bash

# https://www.workplace.com/resources/tech/it-configuration/domain-whitelisting
# https://www.irr.net/docs/list.html
# https://bgp.he.net/search?search%5Bsearch%5D=facebook&commit=Search
# https://github.com/SecOps-Institute/FacebookIPLists/blob/master/facebook_asn_list.lst

set -euo pipefail
set -x

curl https://www.cloudflare.com/ips-v4 > cloudflare/ipv4.txt
curl https://www.cloudflare.com/ips-v6 > cloudflare/ipv6.txt
