#!/bin/bash


# https://www.irr.net/docs/list.html
# https://bgp.he.net/search?search%5Bsearch%5D=pia&commit=Search
# https://github.com/SecOps-Institute/TwitterIPLists/blob/master/pia_asn_list.lst

set -euo pipefail
set -x



curl --silent https://raw.githubusercontent.com/Lars-/PIA-servers/refs/heads/master/export.csv| awk -F','  '{print $1}'| sort -h|uniq >>/tmp/pia.txt


# save ipv4
grep -v ':' /tmp/pia.txt > /tmp/pia-ipv4.txt



# sort & uniq
sort -h /tmp/pia-ipv4.txt | uniq > pia/ipv4.txt
