# Ready-Made BGP Origin Dataset Spike

Date: 2026-06-22
Branch: `bulk-db-augment-plan`

## Goal

Evaluate ready-made observed-BGP origin-prefix datasets/tools before implementing MRT parsing ourselves.

The project needs a local replacement or augmentation for current `riswhois.ripe.net -- "-i origin AS..."` calls. This is separate from IRR/RPSL route objects and separate from ARIN org-net ownership data.

## Sources Evaluated

### RIPE RISWhois Dumps

RIPE documents RISWhois as a data service for "which ASN originates what prefix." It exposes the live whois service at `riswhois.ripe.net`, and the same prefix-to-ASN data is stored in daily dump files:

- `https://www.ris.ripe.net/dumps/riswhoisdump.IPv4.gz`
- `https://www.ris.ripe.net/dumps/riswhoisdump.IPv6.gz`

Source: <https://ris.ripe.net/docs/ris-whois/>

Current metadata checked on 2026-06-22:

```text
riswhoisdump.IPv4.gz: 5,389,780 bytes, Last-Modified Mon, 22 Jun 2026 10:03:45 GMT
riswhoisdump.IPv6.gz: 1,211,025 bytes, Last-Modified Mon, 22 Jun 2026 10:06:13 GMT
```

Pros:

- Exact semantic match for the current `riswhois.ripe.net` source.
- Daily files are small.
- Format is trivial to stream-parse: `origin_asn prefix peer_count`.
- Includes a richer combined-RIS view than a single RouteViews collector.
- No extra tooling needed.

Cons:

- It only covers observed BGP origins, not IRR `route`/`route6`, `mnt-by`, or ownership/org data.
- Some default routes appear and must be filtered.
- It is still one provider's synthesis of RIS collectors, not all RouteViews collectors.

### CAIDA RouteViews pfx2as

CAIDA's public RouteViews pfx2as dataset contains daily IPv4/IPv6 prefix-to-AS mappings derived from RouteViews data. The documented format is line-oriented with tab-separated fields:

- IP prefix
- prefix length
- AS number

Source: <https://www.caida.org/catalog/datasets/routeviews-prefix2as/>

The public directory documents that files are produced from a single RouteViews BGP table snapshot chosen as the median-sized snapshot for that day.

Source: <https://publicdata.caida.org/datasets/routing/>

Current files used:

```text
IPv4: https://publicdata.caida.org/datasets/routing/routeviews-prefix2as/2026/06/routeviews-rv2-20260620-1200.pfx2as.gz
IPv6: https://publicdata.caida.org/datasets/routing/routeviews6-prefix2as/2026/06/routeviews-rv6-20260621-1200.pfx2as.gz
```

Current metadata checked on 2026-06-22:

```text
IPv4 pfx2as: 3,804,031 bytes, Last-Modified Sun, 21 Jun 2026 16:14:07 GMT
IPv6 pfx2as: 882,108 bytes, Last-Modified Mon, 22 Jun 2026 12:04:25 GMT
```

Pros:

- Small daily files.
- Easy automation via `pfx2as-creation.log`.
- Format is even simpler than RISWhois dumps.
- Good independent observed-BGP cross-check from RouteViews.

Cons:

- The public pfx2as files checked here are single-collector RouteViews views, so coverage is lower than RIPE RISWhois for the sample ASNs.
- Multi-origin and AS-set rows need careful parsing. For broad collection, we should include a prefix when the target ASN appears in any `_` or `,` component.
- CAIDA AUA/citation/reporting terms apply.

### Newer CAIDA Prefix-to-AS Dataset

CAIDA announced a newer Prefix-to-AS dataset using BGPStream/bgpview and data from all available BGP collectors from both RouteViews and RIPE RIS. Their blog says it has annotated files and simple files, and the combined collector view covers prefixes not present in the older RouteViews2-only files.

Source: <https://blog.caida.org/best_available_data/2022/11/14/new-caida-prefix-to-as-mapping-data-set/>

This dataset may be a better long-term candidate than the older public `routeviews-prefix2as` files. However, during this spike, the immediately automatable public directory discovered under `publicdata.caida.org/datasets/routing/` exposed the old `routeviews-prefix2as`, `routeviews6-prefix2as`, and stale `routeviews-prefix2as-coalesced` directories. The coalesced directory is experimental and not current; it only lists years 2017-2019.

Decision: track the newer CAIDA Prefix-to-AS dataset as a follow-up research item, but do not block the first implementation on it.

### BGPKIT Parser

BGPKIT Parser is a Rust library and CLI for MRT/BGP/BMP parsing. Its docs advertise streaming parsing, built-in filters, remote file support, transparent decompression, and filters by origin ASN, prefix, peers, time range, AS path, and community. The 2026-05-24 release notes include a route-level parser optimized for prefix, AS path, peer metadata, and timestamp, with large RIB performance improvements.

Source: <https://bgpkit.com/tools/parser/>

Local status:

```text
bgpkit-parser: not installed
cargo: available
rustc: available
```

No install was performed in this spike.

Pros:

- Strong candidate if we later need direct MRT parsing.
- Already Rust-native and optimized for BGP route scans.
- Can filter by origin ASN before downstream processing.

Cons:

- Not needed for the first observed-BGP table if RISWhois dumps are enough.
- Adds a Rust toolchain or binary dependency if the main tool remains Go.
- We need user approval before installing/testing the CLI locally.

### BGPStream / BGPReader

BGPReader is a command-line tool in the BGPStream ecosystem. CAIDA documents that it can access both Route Views and RIPE RIS archives through the default broker interface without manually acquiring files, and can filter records/elements by time and filter strings.

Source: <https://bgpstream.caida.org/docs/tools/bgpreader>

Local status:

```text
bgpreader: not installed
```

No install was performed in this spike.

Pros:

- Mature access layer for both RouteViews and RIPE RIS.
- Can avoid hand-discovering MRT files.
- Good for deeper analysis or future all-collector comparisons.

Cons:

- Much heavier than daily RISWhois/pfx2as text files.
- Not necessary for first shadow implementation.
- Adds native/library dependencies.

## Live Sample Comparison

Sample ASNs:

- `AS714` Apple
- `AS16625` Akamai
- `AS174` Cogent
- `AS32934` Meta/Facebook
- `AS9009` M247

Command shape used:

```bash
python3 - <<'PY'
# streamed gzip files via urllib + gzip, no third-party packages
# parsed RISWhois as: asn prefix peer_count
# parsed CAIDA pfx2as as: prefix length AS
# included CAIDA MOAS/AS-set rows when target ASN appeared in "_" or "," components
PY
```

Rows parsed:

```text
RIS IPv4:   1,251,992 rows in 2.54s
RIS IPv6:     296,632 rows in 0.59s
CAIDA IPv4: 1,101,318 rows in 12.75s
CAIDA IPv6:   256,988 rows in 5.65s
```

Comparison counts:

```text
asn,family,ris,caida,overlap,ris_only,caida_only
AS714,4,1517,1505,1505,12,0
AS16625,4,2482,2194,2194,288,0
AS174,4,4355,4153,4153,202,0
AS32934,4,259,179,179,80,0
AS9009,4,4355,4222,4222,133,0
AS714,6,437,427,427,10,0
AS16625,6,7,6,4,3,2
AS174,6,594,436,435,159,1
AS32934,6,364,266,266,98,0
AS9009,6,810,795,795,15,0
```

Notable samples:

```text
AS714 IPv4 RIS-only: 139.178.140.0/24, 139.178.141.0/24, 17.0.184.0/23
AS16625 IPv4 RIS-only: 104.101.200.0/22, 104.108.64.0/22, 104.109.132.0/22
AS174 IPv4 RIS-only: 104.164.68.0/24, 104.164.69.0/24, 104.164.70.0/24
AS32934 IPv4 RIS-only: 102.132.103.0/24, 102.132.97.0/24, 157.240.12.0/24
AS9009 IPv4 RIS-only: 102.135.10.0/24, 102.135.32.0/24, 102.135.33.0/24
AS16625 IPv6 CAIDA-only: 2404:c000::/32, 2404:c000::/33
AS174 IPv6 CAIDA-only: 2001:550:bb03::/48
```

Interpretation:

- For the five sample ASNs, RIPE RISWhois dumps are a strict superset of the old CAIDA RouteViews pfx2as IPv4 files.
- For IPv6, RISWhois is still much larger, but old CAIDA pfx2as has a few prefixes not present in RISWhois for Akamai and Cogent.
- The older RouteViews pfx2as files are useful as an independent check, but not as the primary first source for replacing `riswhois.ripe.net`.

## Decision

Use **RIPE RISWhois dumps as the first observed-BGP origin-AS source**.

Add optional support for **CAIDA RouteViews pfx2as** as a secondary observed-BGP source in the source config, but do not depend on it for initial parity. Its main value is independent visibility and diff reporting.

Defer **BGPKIT Parser**, **BGPStream/BGPReader**, and raw MRT parsing until reports show real gaps that daily RISWhois + CAIDA pfx2as do not cover.

## Implementation Notes

Initial parser support should handle:

- RISWhois rows: `asn prefix peer_count`
- CAIDA rows: `prefix prefix_length as_field`
- CAIDA AS field variants:
  - single AS: `16625`
  - MOAS: `10_20`
  - AS set or set-like component: `32,54`
  - combined: `10_20_30_32,54`

For broad collection, include a prefix for a target ASN if that ASN appears in any parsed component.

Source freshness fields should record:

- URL
- `Last-Modified`
- `Content-Length`
- fetch time
- parsed row count
- default-route drops

## Recommendation For Next Work

Add the Go-vs-Rust research task to the plan, then run it before implementing `irrindex`. The language decision now matters more because BGPKIT Parser is Rust-native and may influence whether the BGP/MRT side should stay external, become a Rust library dependency, or be deferred behind simple RISWhois/CAIDA text parsers.
