#!/usr/bin/env bash
# Сборка доменных списков: источники -> dist/vpn-domains.{lst,json,srs}
#
# Переменные окружения:
#   SING_BOX_BIN   — путь к бинарю sing-box/amnezia-box для компиляции .srs
#                    (если не задан — .srs не собирается)
#   AGH_DNS_TARGET — например "10.10.3.5:5354": дополнительно генерирует
#                    dist/adguardhome-upstreams.txt для per-domain upstream AGH

set -euo pipefail
cd "$(dirname "$0")"

mkdir -p tmp dist

fetch_into() { # url, аккумулятор
  local url="$1" acc="$2"
  echo ">> $url" >&2
  curl -fsSL --retry 3 --retry-delay 5 --connect-timeout 15 "$url" >> "$acc"
  echo >> "$acc"
}

: > tmp/vpn-all.raw
while IFS= read -r url; do
  [[ -z "${url// /}" || "$url" =~ ^[[:space:]]*# ]] && continue
  fetch_into "$url" tmp/vpn-all.raw
done < sources.txt

: > tmp/exclude-all.raw
if [[ -f exclude-sources.txt ]]; then
  while IFS= read -r url; do
    [[ -z "${url// /}" || "$url" =~ ^[[:space:]]*# ]] && continue
    fetch_into "$url" tmp/exclude-all.raw
  done < exclude-sources.txt
fi

python3 build_lists.py \
  tmp/vpn-all.raw include.txt \
  --exclude-files tmp/exclude-all.raw exclude.txt \
  --out-lst dist/vpn-domains.lst \
  --out-json dist/vpn-domains.json

if [[ -n "${SING_BOX_BIN:-}" ]]; then
  "$SING_BOX_BIN" rule-set compile dist/vpn-domains.json -o dist/vpn-domains.srs
  echo "OK: dist/vpn-domains.srs"
fi

if [[ -n "${AGH_DNS_TARGET:-}" ]]; then
  sed "s/.*/[\/&\/]${AGH_DNS_TARGET//\//\\/}/" dist/vpn-domains.lst > dist/adguardhome-upstreams.txt
  echo "OK: dist/adguardhome-upstreams.txt"
fi
