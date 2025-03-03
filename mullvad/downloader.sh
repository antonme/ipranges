#!/bin/bash


# https://www.irr.net/docs/list.html
# https://bgp.he.net/search?search%5Bsearch%5D=mullvad&commit=Search
# https://github.com/SecOps-Institute/TwitterIPLists/blob/master/mullvad_asn_list.lst

set -euo pipefail
set -x



curl https://api.mullvad.net/app/v1/relays| jq -r "(.openvpn // .wireguard // .bridge).relays[].ipv4_addr_in"|sort -u >/tmp/mullvad-ipv4.txt

curl https://api.mullvad.net/app/v1/relays| jq -r "(.wireguard // .bridge).relays[].ipv6_addr_in"|awk '{print $0"/128"}'|sort -u >/tmp/mullvad-ipv6.txt


# sort & uniq
sort -h /tmp/mullvad-ipv4.txt | uniq > mullvad/ipv4.txt
sort -h /tmp/mullvad-ipv6.txt | uniq > mullvad/ipv6.txt
