#!/bin/bash

# Smallish hostings popular with vpn providers and known vpn providers server locations' ips

set -euo pipefail
set -x


# get from mnt-by
get_maintained() {
    whois -h whois.ripe.net -- "-i mnt-by $1" | rg '^route' | awk '{ print $2; }'
    whois -h whois.radb.net -- "-i mnt-by $1" | rg '^route' | awk '{ print $2; }'
    whois -h rr.ntt.net -- "-i mnt-by $1" | rg '^route' | awk '{ print $2; }'
    whois -h whois.rogerstelecom.net -- "-i mnt-by $1" | rg '^route' | awk '{ print $2; }'
    whois -h whois.bgp.net.br -- "-i mnt-by $1" | rg '^route' | awk '{ print $2; }'
}


# get from Autonomous System
get_routes() {
    whois -h riswhois.ripe.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }'
    whois -h whois.radb.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }'
    whois -h rr.ntt.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }'
    whois -h whois.rogerstelecom.net -- "-i origin $1" | rg '^route' | awk '{ print $2; }'
    whois -h whois.bgp.net.br -- "-i origin $1" | rg '^route' | awk '{ print $2; }'
}


get_maintained 'DATACAMP-MNT' > /tmp/vpnhosts.txt || echo 'failed'
get_maintained 'GLOBALAXS-MNT' >> /tmp/vpnhosts.txt || echo 'failed'
get_maintained 'CLOUVIDER-MNT' >> /tmp/vpnhosts.txt || echo 'failed'
get_maintained 'HYDRA-MNT' >> /tmp/vpnhosts.txt || echo 'failed'
get_maintained 'ALTUSHOST-MNT' >> /tmp/vpnhosts.txt || echo 'failed'
get_maintained 'UKSERVERS-MNT' >> /tmp/vpnhosts.txt || echo 'failed'
get_maintained 'NUXTCLOUD-MNT' >> /tmp/vpnhosts.txt || echo 'failed'
get_maintained 'MAINT-GSLNETWORKS-AU' >> /tmp/vpnhosts.txt || echo 'failed'
get_maintained 'CYBERZONEHUB-MNT' >> /tmp/vpnhosts.txt || echo 'failed'
get_maintained 'MAINT-OWL-VU' >> /tmp/vpnhosts.txt || echo 'failed'
get_maintained 'MNT-HOSTINGINSIDE' >> /tmp/vpnhosts.txt || echo 'failed'
get_maintained 'MNT-WORLDSTREAM' >> /tmp/vpnhosts.txt || echo 'failed'
get_maintained 'fribourg-mnt' >> /tmp/vpnhosts.txt || echo 'failed'
get_maintained 'de-kiservices-1-mnt' >> /tmp/vpnhosts.txt || echo 'failed'
get_maintained 'mnt-de-itmbuechele-1' >> /tmp/vpnhosts.txt || echo 'failed'
get_maintained 'QWARTA-MNT' >> /tmp/vpnhosts.txt || echo 'failed'
get_maintained 'MNT-NFORCE' >> /tmp/vpnhosts.txt || echo 'failed'
get_maintained 'de-buechvps1-1-mnt' >> /tmp/vpnhosts.txt || echo 'failed'
get_maintained 'MAINT-HOSTROYALETECHNOLOGIES-IN' >> /tmp/vpnhosts.txt || echo 'failed'
get_maintained 'OVH-MNT' >> /tmp/vpnhosts.txt || echo 'failed'
get_maintained 'ESAB-MNT' >> /tmp/vpnhosts.txt || echo 'failed'

# CDN77
get_routes 'AS60068' >> /tmp/vpnhosts.txt || echo 'failed'
# Logicweb
get_routes 'AS64286' >> /tmp/vpnhosts.txt || echo 'failed'
# IPXO
get_routes 'AS206092' >> /tmp/vpnhosts.txt || echo 'failed'
# OVH
get_routes 'AS16276' >> /tmp/vpnhosts.txt || echo 'failed'
# ESAB
get_routes 'AS39351' >> /tmp/vpnhosts.txt || echo 'failed'

# known ips from https://github.com/Zomboided/service.vpn.manager.providers
cat vpnhosts-knownips.txt >> /tmp/vpnhosts.txt || echo 'failed'

curl https://raw.githubusercontent.com/antonme/ipnames/master/ext-resolve-vpnhosts.txt >> /tmp/vpnhosts.txt || echo 'failed'

# save ipv4
grep -v ':' /tmp/vpnhosts.txt|rg '\.' > /tmp/vpnhosts-ipv4.txt

# save ipv6
grep ':' /tmp/vpnhosts.txt > /tmp/vpnhosts-ipv6.txt


# sort & uniq
sort -h /tmp/vpnhosts-ipv4.txt | uniq > vpnhosts/ipv4.txt
sort -h /tmp/vpnhosts-ipv6.txt | uniq > vpnhosts/ipv6.txt
