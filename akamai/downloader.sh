#!/usr/bin/env bash


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

python utils/arin-org.py AKAMAI > /tmp/akamai.txt
get_routes 'AS12222' >> /tmp/akamai.txt || echo 'failed'
get_routes 'AS16625' >> /tmp/akamai.txt || echo 'failed'
get_routes 'AS16702' >> /tmp/akamai.txt || echo 'failed'
get_routes 'AS17204' >> /tmp/akamai.txt || echo 'failed'
get_routes 'AS18680' >> /tmp/akamai.txt || echo 'failed'
get_routes 'AS18717' >> /tmp/akamai.txt || echo 'failed'
get_routes 'AS20189' >> /tmp/akamai.txt || echo 'failed'
get_routes 'AS20940' >> /tmp/akamai.txt || echo 'failed'
get_routes 'AS22207' >> /tmp/akamai.txt || echo 'failed'
get_routes 'AS22452' >> /tmp/akamai.txt || echo 'failed'
get_routes 'AS23454' >> /tmp/akamai.txt || echo 'failed'
get_routes 'AS23455' >> /tmp/akamai.txt || echo 'failed'
get_routes 'AS26008' >> /tmp/akamai.txt || echo 'failed'
get_routes 'AS30675' >> /tmp/akamai.txt || echo 'failed'
get_routes 'AS33047' >> /tmp/akamai.txt || echo 'failed'
get_routes 'AS35993' >> /tmp/akamai.txt || echo 'failed'
get_routes 'AS35994' >> /tmp/akamai.txt || echo 'failed'
get_routes 'AS36183' >> /tmp/akamai.txt || echo 'failed'
get_routes 'AS393560' >> /tmp/akamai.txt || echo 'failed'

# save ipv4
grep -v ':' /tmp/akamai.txt > /tmp/akamai-ipv4.txt

# save ipv6
grep ':' /tmp/akamai.txt > /tmp/akamai-ipv6.txt

# sort & uniq
sort -h /tmp/akamai-ipv4.txt | uniq > akamai/ipv4.txt
sort -h /tmp/akamai-ipv6.txt | uniq > akamai/ipv6.txt
