#!/bin/bash

# https://docs.oracle.com/en-us/iaas/Content/General/Concepts/addressranges.htm
# From: https://github.com/nccgroup/cloud_ip_ranges

set -euo pipefail
set -x


# get from public ranges
curl -fsS --retry 3 --retry-delay 5 --retry-all-errors https://docs.oracle.com/en-us/iaas/tools/public_ip_ranges.json > /tmp/oracle.json


# save ipv4
jq '.regions[] | [.cidrs][] | .[].cidr | select(. != null)' -r /tmp/oracle.json > /tmp/oracle-ipv4.txt

# ipv6 not provided


# sort & uniq
sort -h /tmp/oracle-ipv4.txt | uniq > oracle/ipv4.txt
