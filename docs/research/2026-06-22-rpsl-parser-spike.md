# RPSL Parser Spike

Date: 2026-06-22
Branch: `bulk-db-augment-plan`

## Goal

Decide whether the first `irrindex` implementation can use a minimal Go block/attribute parser for the RPSL fields this repo needs, or whether we should depend on a fuller parser such as `rpsl-go`, Rust `rpsl-rs`/`rpsl-parser`, or IRRd's parser.

The repo does not need full routing policy evaluation. It needs source collection for:

- `route` / `route6` prefix
- `origin` ASN
- repeated `mnt-by`
- `inetnum` / `inet6num`
- `source`
- continuation lines, so non-target attributes do not corrupt block parsing

No packages were installed for this spike.

## Decision

Use a **minimal field-preserving Go block parser** for the first implementation, with real fixture tests from RIPE, ARIN IRR, APNIC, and eventually RADb.

Do not use `rpsl-go` as the core parser for v1, because its own documentation says it targets RIPE database responses and is not a full RPSL implementation.

Do not switch the first tool to Rust solely for RPSL parsing, even though Rust has stronger parser libraries. Keep Rust parser libraries as validation references for a later spike if fixtures reveal failures.

Do not treat LACNIC public `lacnic.db.gz` as an initial `route`/`mnt-by` source. In the sampled public dump, LACNIC exposed `inetnum` blocks without `mnt-by`, and no `route`, `route6`, `origin`, or `mnt-by` lines were found by a direct scan.

## Real Dump Sampling

The sample scanner streamed public gzipped dump files with Python stdlib, parsed blocks by blank lines, preserved repeated keys, and attached indented continuation lines to the previous attribute. It stopped after a small number of matching objects per source.

Command shape:

```bash
python3 - <<'PY'
# urllib + gzip only
# split RPSL objects on blank lines
# parse key: value
# preserve repeated attributes as []string
# append indented continuation lines to the previous attribute
PY
```

Summary:

```text
source,status,objects_seen,selected,with_repeated_mnt,with_continuation,first_object_summary
ripe_route,ok,20,20,1,1,route=193.254.30.0/24; origin=AS12726; mnt-by=AS12312-MNT|DE-COLT-MNT|COLT-UK; source=RIPE
ripe_route6,ok,20,20,1,0,route6=2001:1578:200::/40; origin=AS12817; mnt-by=GEFOEKOM-MNT; source=RIPE
ripe_inetnum,ok,20,20,3,0,inetnum=213.159.160.0 - 213.159.191.255; mnt-by=RIPE-NCC-HM-MNT|ERICSSON-MNT; source=RIPE
ripe_inet6num,ok,44,44,1,1,inet6num=2001:638:501::/48; mnt-by=DFN-HM-MNT; source=RIPE
radb,URLError: <urlopen error timed out>,0,0,0,0,
arin_irr,ok,10598,200,0,51,route=2.57.164.0/23; origin=AS46133; mnt-by=MNT-ET-547; source=ARIN
apnic_route,ok,20,20,7,0,route=202.36.121.0/24; origin=AS6831; mnt-by=MAINT-IPROLINK; source=APNIC
apnic_route6,ok,28,28,1,0,route6=2001:4538:41::/48; origin=AS9541; mnt-by=MAINT-PK-CYBERNET; source=APNIC
apnic_inetnum,ok,200,200,0,0,inetnum=202.6.91.0 - 202.6.91.255; mnt-by=APNIC-HM; source=APNIC
apnic_inet6num,ok,20,20,1,0,inet6num=2001:200:200::/40; mnt-by=MAINT-JP-WIDE; source=APNIC
lacnic,ok,14403,200,0,0,inetnum=5.183.80/22; source=LACNIC
```

RADb retry:

```text
curl --max-time 20 -fsSI https://ftp.radb.net/radb/dbase/radb.db.gz
curl: (28) Connection timed out after 20005 milliseconds

curl --max-time 25 -fsSL https://ftp.radb.net/radb/dbase/radb.db.gz | gzip -dc
curl: (28) Connection timed out after 25004 milliseconds
gzip: (stdin): unexpected end of file
```

RADb timeout is a source-fetch reliability issue, not a parser finding. The implementation should keep RADb in the source list, but with retries and previous-snapshot fallback.

## Important Sample Blocks

### RIPE route with continuation lines

```text
route:          194.113.52.0/23
descr:          BEUMER
descr:          BEUMER
origin:         AS702
member-of:      AS702:RS-DE,
                AS702:RS-DE-PI
mnt-by:         WCOM-EMEA-RICE-MNT
created:        2001-11-23T11:58:37Z
last-modified:  2010-11-12T12:58:37Z
source:         RIPE
remarks:        ****************************
remarks:        * THIS OBJECT IS MODIFIED
remarks:        * Please note that all data that is generally regarded as personal
remarks:        * data has been removed from this object.
remarks:        * To view the original object, please query the RIPE Database at:
remarks:        * http://www.ripe.net/whois
remarks:        ****************************
```

Parser implication: continuation lines must be attached to the previous key, but we only need target keys for indexing.

### ARIN route with continuation lines

```text
route:          4.1.82.16/28
origin:         AS18676
descr:          211 Mount Airy Road
                Basking Ridge NJ 07920
                United States
admin-c:        PHILL1092-ARIN
tech-c:         AVAYA-ARIN
tech-c:         PHILL1092-ARIN
mnt-by:         MNT-AVAYAC
created:        2022-05-11T16:04:42Z
last-modified:  2025-04-04T14:34:58Z
source:         ARIN
```

Parser implication: ARIN has normal `route`, `origin`, `mnt-by`, and `source`, and also has continuation lines in non-target fields.

### LACNIC inetnum format

```text
inetnum:    5.183.80/22
status:     allocated
city:       Santo Domingo
country:    DO
created:    2024-07-11
changed:    2024-07-11
source:     LACNIC
```

Parser implication: this is not a RIPE/APNIC-style `start - end` range, and it has no `mnt-by`. It is not directly useful for current `get_maintained` emulation.

### LACNIC encoding issue

The first LACNIC sample attempt with default awk failed on non-UTF-8/locale handling:

```text
awk: towc: multibyte conversion failure on: '�o Paulo
country: BR
created: 1999-11-12
changed: 2013-03-06
source:  LACNIC'
```

Parser implication: process dumps as bytes or decode UTF-8 with replacement. Only trust normalized target fields.

## Minimal Parser Requirements

The v1 Go parser should:

- read from `io.Reader`
- support gzip streaming outside the parser
- split objects on blank lines
- ignore full-line comments beginning with `%` or `#`
- parse the first `key: value` line of each attribute
- preserve repeated attributes as `map[string][]string`
- append indented continuation lines to the immediately previous attribute
- preserve unknown attributes without interpretation
- decode invalid UTF-8 with replacement or operate on bytes and normalize target fields
- normalize keys to lowercase
- preserve target values exactly before later normalization

Target object extraction:

- `route`: prefix from `route`, origin from `origin`, maintainers from all `mnt-by`
- `route6`: prefix from `route6`, origin from `origin`, maintainers from all `mnt-by`
- `inetnum`: range from `inetnum`, maintainers from all `mnt-by`
- `inet6num`: prefix/range from `inet6num`, maintainers from all `mnt-by`

Target range conversion rules:

- RIPE/APNIC IPv4 `inetnum`: parse `start - end` and convert to CIDRs.
- RIPE/APNIC IPv6 `inet6num`: usually already CIDR-like; validate as IPv6 prefix.
- LACNIC `inetnum`: defer unless we identify a useful maintainer or ownership key. It is not needed for current `mnt-by` emulation.

## Parser Options

### Minimal Go Parser

Pros:

- Sufficient for the fields sampled in RIPE, ARIN IRR, APNIC, and LACNIC.
- Easy to keep dependency-light.
- Easy to test with real fixture blocks.
- Easy to integrate with the Go/SQLite `irrindex` plan.
- Avoids parser libraries that may parse policy grammar we do not need.

Risks:

- Not a full RPSL implementation.
- Needs explicit tests for continuation lines and repeated attributes.
- Needs real RADb fixtures once RADb fetch succeeds.
- Needs byte-safe handling for non-UTF-8 text.

### `github.com/frederic-arr/rpsl-go`

The package documentation says it parses RIPE database responses and is not a full implementation of RPSL.

Sources:

- <https://pkg.go.dev/github.com/frederic-arr/rpsl-go>
- <https://github.com/frederic-arr/rpsl-go>

Pros:

- Go-native.
- May be useful as a reference for RIPE-style output.

Cons:

- Its scope is narrower than our multi-registry dump parsing.
- It is not full RPSL.
- It does not remove the need for fixture validation across ARIN/APNIC/LACNIC/RADb.

Decision: do not depend on it for v1.

### Rust `rpsl-rs` / Python `rpsl-parser`

`rpsl-rs` describes itself as a Routing Policy Specification Language parser focused on speed and correctness, including multiline values, whois responses, low memory footprint, property-based tests, and optional validation.

`rpsl-parser` on PyPI advertises an RFC 2622 conformant parser with the same lineage.

Sources:

- <https://docs.rs/rpsl-rs>
- <https://crates.io/crates/rpsl-rs>
- <https://pypi.org/project/rpsl-parser/>

Pros:

- Stronger correctness story.
- Better multiline handling claims.
- Useful validation reference.

Cons:

- Pulling it into v1 conflicts with the Go decision.
- It parses more of RPSL than we need.
- Python package would add a second runtime/dependency path unless used only in research via `uv`.

Decision: keep as validation/reference tools if the minimal Go parser fails real fixtures.

### IRRd Parser

IRRd's docs say all RPSL parsing and validation happens in `irrd.rpsl`, with a class for each supported RPSL object class and metadata extraction such as relevant prefix or origin from route objects.

Source: <https://irrd.readthedocs.io/en/stable/development/architecture/>

Pros:

- Production-grade IRR parser.
- Extracts exactly the kind of metadata we care about.
- Handles many real-world deviations.

Cons:

- IRRd itself needs PostgreSQL and Redis for the full application.
- Local run was blocked in the previous IRRd spike.
- Using just the internal parser would tie us to a large Python project internals.

Decision: use IRRd as a reference backend in a later Docker-backed spike, not as a v1 parser dependency.

## Plan Adjustments

Recommended changes to the implementation plan:

- Keep `radb` in the source list, but mark fetch reliability as a first-class concern with retries and previous-snapshot fallback.
- Keep `ripe_route`, `ripe_route6`, `ripe_inetnum`, `ripe_inet6num`, `arin_irr`, `apnic_route`, `apnic_route6`, `apnic_inetnum`, and `apnic_inet6num` in the first parser fixture set.
- Defer `lacnic` as an origin/mnt source until a useful public LACNIC IRR route source is identified. The sampled public DB did not contain `route`, `route6`, `origin`, or `mnt-by`.
- Add parser tests for invalid UTF-8/encoding replacement.
- Add parser tests for continuation lines on non-target attributes.
- Add parser tests for repeated `mnt-by`.

## Final Recommendation

Proceed with a minimal Go RPSL block parser for v1.

Make the parser deliberately conservative:

- preserve target fields exactly
- index only objects that have the required keys for a query type
- skip malformed objects
- count skipped objects by source and reason
- validate with real fixture blocks before using it in shadow reports

The parser is not the main architectural risk if we keep it field-focused and fixture-driven. The larger risks are source fetch reliability, source freshness, and comparison/reporting discipline.
