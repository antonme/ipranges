#!/bin/bash

set -euo pipefail
set -x


# get from mnt
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


#government
get_maintained 'TFGS-MNT' > /tmp/rugov.txt || echo 'failed'
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
#ddos-guard (RU-LLCDDOS)
get_maintained 'DDOS-GUARD-5-MNT' >> /tmp/rugov.txt || echo 'failed'
get_maintained 'DDOS-GUARD-MNT3' >> /tmp/rugov.txt || echo 'failed'
#esia.gosuslugi.ru,digital.gov.ru (Electronic-government)
get_routes AS196747 >> /tmp/rugov.txt || echo 'failed'
#mvd gtssizi
get_routes AS44560 >> /tmp/rugov.txt || echo 'failed'
#ffoms
get_routes AS57145 >> /tmp/rugov.txt || echo 'failed'
#minek
get_routes AS42056 >> /tmp/rugov.txt || echo 'failed'
#rosim
get_routes AS31379 >> /tmp/rugov.txt || echo 'failed'
#rostransnadzor
get_routes AS198688 >> /tmp/rugov.txt || echo 'failed'
#ctspi mga
get_routes AS34500 >> /tmp/rugov.txt || echo 'failed'
#mid
get_routes AS57580 >> /tmp/rugov.txt || echo 'failed'
#goznak
get_routes AS56725 >> /tmp/rugov.txt || echo 'failed'
#regional e-gov: spb, tatarstan, sverdlovsk, tyumen, amur, chuvashia, penza
get_routes AS203725 >> /tmp/rugov.txt || echo 'failed'
get_routes AS34887 >> /tmp/rugov.txt || echo 'failed'
get_routes AS201643 >> /tmp/rugov.txt || echo 'failed'
get_routes AS196894 >> /tmp/rugov.txt || echo 'failed'
get_routes AS39562 >> /tmp/rugov.txt || echo 'failed'
get_routes AS56581 >> /tmp/rugov.txt || echo 'failed'
get_routes AS13029 >> /tmp/rugov.txt || echo 'failed'
get_routes AS56613 >> /tmp/rugov.txt || echo 'failed'
#regional governments: sakhalin, kirov, magadan, khabarovsk, kurgan
get_routes AS196721 >> /tmp/rugov.txt || echo 'failed'
get_routes AS197668 >> /tmp/rugov.txt || echo 'failed'
get_routes AS208605 >> /tmp/rugov.txt || echo 'failed'
get_routes AS57637 >> /tmp/rugov.txt || echo 'failed'
get_routes AS51882 >> /tmp/rugov.txt || echo 'failed'

# save ipv4
awk '$0 !~ /:/ && $0 != "0.0.0.0/0" { print }' /tmp/rugov.txt > /tmp/rugov-ipv4.txt

# save ipv6
awk '$0 ~ /:/ && $0 != "::/0" { print }' /tmp/rugov.txt > /tmp/rugov-ipv6.txt


# sort & uniq
sort -h /tmp/rugov-ipv4.txt | uniq > rugov/ipv4.txt
sort -h /tmp/rugov-ipv6.txt | uniq > rugov/ipv6.txt
