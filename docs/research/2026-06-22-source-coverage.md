# IRR Source Coverage Expansion

Date: 2026-06-22
Branch: `bulk-db-augment-plan`

## Goal

Improve shadow index coverage for the IRR mirrors that current downloader scripts query most often:

- `rr.ntt.net`
- `whois.rogerstelecom.net`
- `whois.bgp.net.br`

## Sources Added

### NTTCOM

Added source name: `nttcom`

URL:

```text
https://rr1.ntt.net/nttcomRR/nttcom.db.gz
```

Evidence:

- The public `rr1.ntt.net` index lists `nttcom.db.gz`.
- The IRR registry list identifies `NTTCOM` import source as `https://rr1.ntt.net/nttcomRR/`.
- A local `curl -I` check on 2026-06-22 returned HTTP 200, content length `2636778`, and `Last-Modified: Mon, 22 Jun 2026 17:43:20 GMT`.

Decision: include as an RPSL source with `route` and `route6` objects.

### TC / bgp.net.br

Added source name: `bgp_net_br`

URL:

```text
https://ftp.bgp.net.br/tc.db.gz
```

Evidence:

- The public `ftp.bgp.net.br` index lists `tc.db.gz`.
- The IRR registry list identifies `TC` whois as `whois.bgp.net.br` and import source as `ftp://ftp.bgp.net.br/`.
- A local `curl -I` check on 2026-06-22 returned HTTP 200, content length `2905201`, and `Last-Modified: Mon, 22 Jun 2026 18:25:38 GMT`.

Decision: include as an RPSL source with `route` and `route6` objects.

## Rogers Status

No standalone Rogers HTTPS dump was confirmed.

Observed:

- The current repo queries `whois.rogerstelecom.net` directly in many downloaders.
- The IRR registry list shows `TC` mirrors `ROGERS`.
- Direct HTTPS probes for likely Rogers dump paths either timed out or did not return a confirmed downloadable RPSL gzip.
- The current Go fetcher supports HTTP(S), not FTP.

Decision: do not add a standalone `rogers` source until a reliable public HTTP(S) dump is confirmed or FTP support is intentionally added. Do not map `rogers` queries to the entire `bgp_net_br` dump yet, because the store currently filters by dump source name rather than by each object's internal `source:` attribute.

## Follow-Up

If Rogers coverage remains important after the next shadow compare:

1. Preserve object-level `source:` attributes as queryable source aliases.
2. Then allow `rogers` mirror filters to match `source: ROGERS` objects inside aggregate dumps such as TC.
3. Only add FTP fetch support if no HTTP(S) source exists and the workflow cost is justified.
