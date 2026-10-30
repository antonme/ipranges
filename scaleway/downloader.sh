#!/bin/bash

# https://www.scaleway.com/en/docs/account/support/scaleway-network-information/
# Scaleway SAS (RIPE org ORG-TT1-RIPE) runs its cloud/dedibox network on AS12876
# (Online S.A.S.). The org's other ASNs are not Scaleway: AS29447 is Iliad Italia,
# AS202023 is Free/Proxad and AS205537 is Ecole 42.

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

# official list, published as Markdown in Scaleway's docs repo; required, so a
# failed fetch or an empty section aborts before the old lists are overwritten
curl -fLsS --retry 3 --retry-delay 5 --retry-all-errors https://raw.githubusercontent.com/scaleway/docs-content/main/pages/account/support/scaleway-network-information.mdx \
    | sed -n '/^## IP ranges used by Scaleway/,/^## /p' \
    | rg -o '^[*+-][[:space:]]+`([^`]+/[0-9]+)`' -r '$1' > /tmp/scaleway.txt
test -s /tmp/scaleway.txt

get_routes 'AS12876' >> /tmp/scaleway.txt

# keep only prefixes and drop any default route an origin query might surface;
# a /0 would swallow the merged list.
grep "/" /tmp/scaleway.txt | grep -vE '^(0\.0\.0\.0|::)/0$' > /tmp/scaleway-clean.txt

# save ipv4
grep -v ':' /tmp/scaleway-clean.txt > /tmp/scaleway-ipv4.txt

# save ipv6
grep ':' /tmp/scaleway-clean.txt > /tmp/scaleway-ipv6.txt


# sort & uniq
sort -h /tmp/scaleway-ipv4.txt | uniq > scaleway/ipv4.txt
sort -h /tmp/scaleway-ipv6.txt | uniq > scaleway/ipv6.txt
