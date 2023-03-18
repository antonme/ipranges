#!/bin/bash

set -euo pipefail
set -x


# get from mnt
get_maintained() {
    whois -h whois.ripe.net -- "-i mnt-by $1" | grep '^route' | awk '{ print $2; }'
    whois -h whois.radb.net -- "-i mnt-by $1" | grep '^route' | awk '{ print $2; }'
    whois -h rr.ntt.net -- "-i mnt-by $1" | grep '^route' | awk '{ print $2; }'
    whois -h whois.rogerstelecom.net -- "-i mnt-by $1" | grep '^route' | awk '{ print $2; }'
    whois -h whois.bgp.net.br -- "-i mnt-by $1" | grep '^route' | awk '{ print $2; }'
}

# get from Autonomous System
get_routes() {
    whois -h riswhois.ripe.net -- "-i origin $1" | grep '^route' | awk '{ print $2; }'
    whois -h whois.radb.net -- "-i origin $1" | grep '^route' | awk '{ print $2; }'
    whois -h rr.ntt.net -- "-i origin $1" | grep '^route' | awk '{ print $2; }'
    whois -h whois.rogerstelecom.net -- "-i origin $1" | grep '^route' | awk '{ print $2; }'
    whois -h whois.bgp.net.br -- "-i origin $1" | grep '^route' | awk '{ print $2; }'
}


#government
get_maintained 'TFGS-MNT' >> /tmp/rugov.txt || echo 'failed'
get_maintained 'INFR-MNT' >> /tmp/rugov.txt || echo 'failed'
get_maintained 'ROSNIIROS-MNT' >> /tmp/rugov.txt || echo 'failed'
#mchs
get_maintained 'mnt-ru-servguru-1' >> /tmp/rugov.txt || echo 'failed'
#mos
get_routes AS62268 >> /tmp/rugov.txt || echo 'failed'
get_routes AS8901 >> /tmp/rugov.txt || echo 'failed'
#mil
get_routes AS33972 >> /tmp/rugov.txt || echo 'failed'
#minzdrav
get_routes AS199148 >> /tmp/rugov.txt || echo 'failed'
#economy
get_routes AS12389 >> /tmp/rugov.txt || echo 'failed'
#digital
get_maintained 'SERVICEPIPE-MNT' >> /tmp/rugov.txt || echo 'failed'
#mvd,transport.mos.ru
get_routes AS209030 >> /tmp/rugov.txt || echo 'failed'
#fsb
get_routes AS8342 >> /tmp/rugov.txt || echo 'failed'
#vgtrk
get_maintained 'VGTRK-MNT' >> /tmp/rugov.txt || echo 'failed'
#fss
get_maintained 'FSS-MNT' >> /tmp/rugov.txt || echo 'failed'
#fsin+
get_maintained 'RUWEB-MNT-RIPE' >> /tmp/rugov.txt || echo 'failed'

# save ipv4
grep -v ':' /tmp/rugov.txt > /tmp/rugov-ipv4.txt

# save ipv6
grep ':' /tmp/rugov.txt > /tmp/rugov-ipv6.txt


# sort & uniq
sort -h /tmp/rugov-ipv4.txt | uniq > rugov/ipv4.txt
sort -h /tmp/rugov-ipv6.txt | uniq > rugov/ipv6.txt
