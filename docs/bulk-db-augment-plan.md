# Bulk DB Augment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a local snapshot/index pipeline for public routing databases that first shadows and compares the existing whois-based downloaders, then safely augments them, and only later replaces selected live whois calls after sustained evidence.

**Architecture:** Add a separate Go-based `irrindex` tool and a separate shadow workflow. The current `update.yml`, downloader scripts, generated `ipv4.txt`/`ipv6.txt`, merged outputs, `utils/arin-org.py`, and sanity-floor behavior stay unchanged until explicit later migration tasks.

**Tech Stack:** Go 1.22+, SQLite, gzip streaming, shell wrappers for optional downloader integration, existing Python `netaddr` utilities for current merge behavior.

## Global Constraints

- Preserve current auto-updates: do not change `.github/workflows/update.yml` in the first implementation phase.
- Do not modify committed service outputs (`*/ipv4.txt`, `*/ipv6.txt`, `*_merged.txt`) during shadow mode.
- Treat the project as a broad source collector, not an authoritative routing validator.
- Keep ARIN Whois-RWS org-net lookup live initially; do not use ARIN Bulk Whois unless the user explicitly accepts ARIN's approval and Terms of Use path.
- Store raw dumps and built indexes outside normal git history: GitHub Actions cache/artifacts or release assets only.
- FTP source downloads are allowed for mirrors whose HTTPS endpoints are broken; the Go fetcher shells out to `curl` for `ftp://` URLs.
- Local query tools must return exit code `0` for "no results" so they do not break existing `set -euo pipefail` downloader patterns.
- Filter `0.0.0.0/0` and `::/0` in the indexer and keep existing downloader guards intact.
- Preserve source provenance in reports: local output must be explainable by source, object type, ASN/maintainer, and snapshot date.

---

## Independent Plans Reviewed

Four independent plans were considered before this synthesis.

- Codex plan: branch-first, augment-first, source inventory grounded in the repo's 58 downloaders, shadow reports before any output change, `origin -> prefixes`, `mnt-by -> prefixes`, and inetnum support as first-class requirements.
- Opus plan: emphasized "shadow -> augment -> trim", separate snapshot artifacts, row-count safety gates, `irrdb` CLI with `fetch/build/verify/query/diff/compare`, and no ARIN Bulk Whois in the first phase.
- Gemini plan: emphasized streaming Go ingestion, SQLite schema, GitHub Actions cache/artifacts, local-output comparison, and coverage reports.
- GLM 5.2 plan: emphasized not touching `update.yml` initially, a separate `irr-snapshot.yml`, explicit cutover criteria, a whois-compatible shim, multi-valued `mnt-by`, and `queries.yaml` as the stable contract between bash downloaders and the local index.

The synthesized plan uses GLM's separate shadow workflow and cutover rules, Opus's retention/safety gates, Gemini's streaming Go implementation, and Codex's repo-specific source inventory.

## Existing Tools Research

This section records web research performed before implementation, so the custom `irrindex` path is not chosen blindly.

### IRRd v4

IRRd v4 is the closest existing match. It is an open-source Internet Routing Registry daemon that processes RPSL objects, validates/cleans/stores IRR data, extracts indexed metadata, supports file imports and NRTM mirroring, and exposes query interfaces. It supports both raw port-43/HTTPS whois queries and GraphQL queries.

Relevant capabilities:

- Mirrors other IRR databases using file imports and NRTM.
- Stores indexed route metadata including route prefixes and origins.
- Supports RIPE-style `-i origin AS...` and `-i mnt-by ...` queries.
- Supports IRRd-style `!gAS...` and `!6AS...` origin-prefix queries.
- Supports source filters such as `-s RIPE,NTTCOM`.
- Exposes `databaseStatus` over GraphQL with per-source mirror freshness/status fields.
- Exposes `asnPrefixes(asns: ..., ipVersion: ..., sources: ...)` over GraphQL.
- Treats multi-valued fields like `mnt-by` as arrays.

Reasons to consider using IRRd:

- It already implements RPSL parsing, mirroring, source status, whois compatibility, and GraphQL.
- RADb explicitly recommends private WHOIS/NRTM mirroring to reduce public server load and keep local copies consistent.
- It may let us replace the planned `whois-shim` with a real local IRRd instance and keep downloader query syntax very close to today.

Reasons not to make IRRd the default first implementation:

- It requires PostgreSQL and Redis, and IRRd docs note the PostgreSQL DB is roughly three times the imported RPSL text size.
- It is service-oriented, which is heavier for GitHub Actions than a single static binary plus SQLite artifact.
- It solves IRR data, not RIPE RISWhois dumps, CAIDA pfx2as, ARIN Whois-RWS org nets, or repo-specific comparison reports.
- The project needs a reproducible shadow artifact and per-service reports more than a permanent public registry daemon.

Decision: keep the Go/SQLite `irrindex` plan as the first branch deliverable, but add an early spike task to run IRRd locally against a small source set and compare complexity/results. If IRRd proves simple enough in CI, it can replace or shrink parts of `irrindex`.

### bgpq4

`bgpq4` is an open-source IRR prefix-list generator. It can query an IRRd host, generate IPv4/IPv6 prefix lists, aggregate output, emit JSON, and generate MikroTik RouterOS output.

Reasons to use it:

- Useful as a cross-check for origin/as-set prefix generation.
- Already supports `-h host[:port]` for selecting a local IRRd server.
- Can generate JSON or RouterOS formats if we later want router-native artifacts.

Reasons not to use it as the core:

- It is primarily a filter/config generator, not a snapshot builder.
- It still expects an IRR query backend such as IRRd.
- Our repo needs `mnt-by`, inetnum-derived CIDRs, broad source provenance, comparison reports, and union-with-existing-downloaders behavior.

Decision: use `bgpq4` as an optional verification tool in the IRRd spike, not as the core local database.

### BGP/RIS/RouteViews Tools And Datasets

Several open-source or ready-made options can cover observed BGP origin-prefix data:

- RIPE RISWhois dumps are already small and simple enough to parse directly.
- CAIDA RouteViews prefix-to-AS files are daily line-oriented mappings with `prefix`, `prefix length`, and `AS number`.
- BGPKIT Parser is an open-source Rust MRT/BGP parser with a CLI that can filter by origin AS or prefix and stream/cache remote data.
- BGPStream/BGPReader is an open-source framework that can access Route Views and RIPE RIS archives without manually discovering files.
- `mrtparse` is a Python MRT parser.

Decision: keep RISWhois dumps in the first source set because they are small and match current `riswhois.ripe.net` semantics. Add CAIDA pfx2as as a high-priority research candidate because it may provide a ready-made daily observed-BGP origin table without MRT parsing. Defer BGPKIT/BGPStream/MRT parsing unless RISWhois and CAIDA pfx2as leave real gaps.

### RPSL Parser Libraries

Existing parsers include:

- `rpsl-go`: Go parser for RIPE database responses; explicitly not a full RPSL implementation and does not validate object or attribute values.
- `rpsl-parser`: Python/Rust-backed parser advertised as RFC 2622 conformant, fast, low-memory, and capable of parsing whois responses.
- `rpsl4j-parser`: Java parser forked from the RIPE NCC whois project.
- IRRd's own Python RPSL parser, which its docs describe as the single source of truth for object structure inside IRRd.

Decision: do not hand-roll a parser beyond the minimal block/attribute parser unless fixtures show it is enough. Task 2 should include a short dependency spike comparing minimal Go parsing, `rpsl-go`, and IRRd's parser behavior on real RIPE, RADb, ARIN IRR, APNIC, and LACNIC samples.

### ARIN IRR And ARIN Org Nets

ARIN's IRR REST docs confirm route/route6 objects use `route`/`route6`, `origin`, `mnt-by`, and `source: ARIN`; these are suitable for the local IRR index. The docs also show ARIN `mnt-by` is in `MNT-OrgID` format for new IRR records, and is not a substitute for the existing Whois-RWS org-net lookup in `utils/arin-org.py`.

Decision: index ARIN IRR route objects locally, but leave `utils/arin-org.py` untouched until a later, separate design decision.

## Current Repo Baseline

The existing repo has these observed properties:

- 58 `downloader.sh` scripts.
- 26 services use origin-AS whois lookups.
- 20 services use `mnt-by` whois lookups.
- 12 services use ARIN org handles through `utils/arin-org.py`.
- 10 services convert `inetnum` ranges through `utils/ipcalc.py`.
- 30 services also use HTTP/vendor/ipnames feeds.
- Estimated per-run whois fanout from static scripts: about 145 RADb calls, 145 NTT calls, 135 Rogers calls, 135 `bgp.net.br` calls, 95 RISWhois calls, 50 RIPE calls, plus APNIC direct calls.

The implementation must therefore support both origin-AS and maintainer lookups. RIS/BGP-only replacement is useful but insufficient by itself.

## Target File Structure

Create these files and directories:

```text
irrindex/
  cmd/irrindex/main.go
  internal/config/config.go
  internal/fetch/fetch.go
  internal/rpsl/parser.go
  internal/ris/parser.go
  internal/store/schema.sql
  internal/store/store.go
  internal/query/query.go
  internal/compare/compare.go
  internal/manifest/manifest.go
  config/sources.yaml
  config/queries.yaml
  testdata/rpsl/ripe-route.txt
  testdata/rpsl/route6-multi-mnt.txt
  testdata/ris/riswhoisdump-ipv4.txt
  testdata/fixtures/downloader-live.txt
docs/superpowers/plans/2026-06-22-bulk-db-augment.md
.github/workflows/irr-snapshot.yml
```

Do not create or commit:

```text
snapshots/*.gz
snapshots/*.db
indexes/*.db
*/ipv4_local.txt
*/ipv6_local.txt
```

If local generated files are useful during development, add them to `.gitignore` in a later implementation task.

## Data Sources

Initial source set:

```yaml
sources:
  ris_ipv4:
    url: "https://www.ris.ripe.net/dumps/riswhoisdump.IPv4.gz"
    format: "riswhois"
    family: 4
  ris_ipv6:
    url: "https://www.ris.ripe.net/dumps/riswhoisdump.IPv6.gz"
    format: "riswhois"
    family: 6
  ripe_route:
    url: "https://ftp.ripe.net/ripe/dbase/split/ripe.db.route.gz"
    format: "rpsl"
    object_types: ["route"]
  ripe_route6:
    url: "https://ftp.ripe.net/ripe/dbase/split/ripe.db.route6.gz"
    format: "rpsl"
    object_types: ["route6"]
  ripe_inetnum:
    url: "https://ftp.ripe.net/ripe/dbase/split/ripe.db.inetnum.gz"
    format: "rpsl"
    object_types: ["inetnum"]
  ripe_inet6num:
    url: "https://ftp.ripe.net/ripe/dbase/split/ripe.db.inet6num.gz"
    format: "rpsl"
    object_types: ["inet6num"]
  radb:
    url: "ftp://ftp.radb.net/radb/dbase/radb.db.gz"
    format: "rpsl"
    object_types: ["route", "route6"]
  nttcom:
    url: "https://rr1.ntt.net/nttcomRR/nttcom.db.gz"
    format: "rpsl"
    object_types: ["route", "route6"]
  bgp_net_br:
    url: "https://ftp.bgp.net.br/tc.db.gz"
    format: "rpsl"
    object_types: ["route", "route6"]
  arin_irr:
    url: "https://ftp.arin.net/pub/rr/arin.db.gz"
    format: "rpsl"
    object_types: ["route", "route6"]
  apnic_route:
    url: "https://ftp.apnic.net/apnic/whois/apnic.db.route.gz"
    format: "rpsl"
    object_types: ["route"]
  apnic_route6:
    url: "https://ftp.apnic.net/apnic/whois/apnic.db.route6.gz"
    format: "rpsl"
    object_types: ["route6"]
  apnic_inetnum:
    url: "https://ftp.apnic.net/apnic/whois/apnic.db.inetnum.gz"
    format: "rpsl"
    object_types: ["inetnum"]
  apnic_inet6num:
    url: "https://ftp.apnic.net/apnic/whois/apnic.db.inet6num.gz"
    format: "rpsl"
    object_types: ["inet6num"]
```

Deferred source set:

- LACNIC DB: sampled `lacnic.db.gz` exposed `inetnum` records without `mnt-by` and no `route`/`route6`/`origin`/`mnt-by` lines in a direct scan, so defer until a useful public LACNIC IRR source is identified.
- AFRINIC DB: add only after confirming fetch path, access constraints, and whether redistribution/use terms fit this project.
- Rogers DB: `whois.rogerstelecom.net` is queried by current scripts, and TC lists `ROGERS` among mirrored databases, but no standalone Rogers HTTP(S) dump has been confirmed. Defer direct Rogers until either an HTTP(S) dump is found or object-level `source:` filtering lets `rogers` match `source: ROGERS` objects inside aggregate dumps.
- RouteViews/RIS MRT: add only if RISWhois dumps and IRR dumps leave repeated important gaps.
- ARIN Bulk Whois: do not add in this plan.

## SQLite Schema

Use SQLite because it is easy to inspect in CI artifacts, easy to query from Go, and simple to ship as a single file.

```sql
CREATE TABLE sources (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  url TEXT NOT NULL,
  fetched_at TEXT NOT NULL,
  last_modified TEXT,
  sha256 TEXT NOT NULL,
  size_bytes INTEGER NOT NULL,
  ok INTEGER NOT NULL,
  error TEXT
);

CREATE TABLE objects (
  id INTEGER PRIMARY KEY,
  source_id INTEGER NOT NULL REFERENCES sources(id),
  object_type TEXT NOT NULL,
  family INTEGER NOT NULL,
  prefix TEXT,
  start_ip TEXT,
  end_ip TEXT
);

CREATE TABLE attrs (
  object_id INTEGER NOT NULL REFERENCES objects(id),
  name TEXT NOT NULL,
  value TEXT NOT NULL
);

CREATE TABLE prefixes (
  source_id INTEGER NOT NULL REFERENCES sources(id),
  family INTEGER NOT NULL,
  prefix TEXT NOT NULL,
  origin_asn TEXT,
  mnt_by TEXT,
  object_type TEXT NOT NULL
);

CREATE UNIQUE INDEX ux_prefixes_source_tuple
  ON prefixes(source_id, family, prefix, COALESCE(origin_asn, ''), COALESCE(mnt_by, ''), object_type);

CREATE INDEX ix_prefixes_origin
  ON prefixes(origin_asn, family);

CREATE INDEX ix_prefixes_mnt_by
  ON prefixes(mnt_by, family);

CREATE INDEX ix_prefixes_source_family
  ON prefixes(source_id, family);
```

Important parsing rules:

- `mnt-by` is multi-valued; create one `prefixes` row for each maintainer value.
- `route` and `route6` create rows keyed by `origin_asn` and every `mnt-by`.
- `inetnum` and `inet6num` create maintainer-keyed rows after range-to-CIDR conversion.
- `riswhois` rows create origin-AS rows with no maintainer.
- Drop default routes during build and report how many were dropped.
- Store source names in normalized form such as `ripe`, `radb`, `arin_irr`, `apnic`, and `ris`.

## CLI Contract

The binary name is `irrindex`.

```bash
irrindex fetch --config irrindex/config/sources.yaml --cache-dir "$RUNNER_TEMP/irr-cache" --manifest "$RUNNER_TEMP/irr-cache/manifest.json"
irrindex build --config irrindex/config/sources.yaml --cache-dir "$RUNNER_TEMP/irr-cache" --db "$RUNNER_TEMP/irrindex.db"
irrindex verify --db "$RUNNER_TEMP/irrindex.db" --previous-manifest snapshots/manifest.json
irrindex query origin AS16625 --db "$RUNNER_TEMP/irrindex.db" --family 4
irrindex query origin AS16625 --db "$RUNNER_TEMP/irrindex.db" --family 6
irrindex query mnt-by DATACAMP-MNT --db "$RUNNER_TEMP/irrindex.db" --family 4
irrindex compare --db "$RUNNER_TEMP/irrindex.db" --queries irrindex/config/queries.yaml --live-root . --out "$RUNNER_TEMP/irr-report"
irrindex whois-shim --db "$RUNNER_TEMP/irrindex.db" -h whois.radb.net -- "-i origin AS16625"
```

Output contract:

- `query` prints one CIDR per line on stdout.
- `query` exits `0` for empty results.
- `query` logs parser/source diagnostics only to stderr.
- `whois-shim` accepts the existing whois invocation shape and can be used later for minimal downloader diffs.

## Task 1: Go Module And Config Skeleton

**Files:**
- Create: `go.mod`
- Create: `irrindex/cmd/irrindex/main.go`
- Create: `irrindex/config/sources.yaml`
- Create: `irrindex/config/queries.yaml`
- Create: `irrindex/internal/config/config.go`
- Test: `irrindex/internal/config/config_test.go`

**Interfaces:**
- Produces: `config.LoadSources(path string) (SourcesConfig, error)`
- Produces: `config.LoadQueries(path string) (QueriesConfig, error)`
- Produces: CLI subcommands `fetch`, `build`, `verify`, `query`, `compare`, `whois-shim`

- [ ] **Step 1: Write config loader tests**

Create tests that load one source and one query from inline YAML fixtures. Assert source name, URL, format, object types, downloader, kind, value, and mirrors.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./irrindex/internal/config -run TestLoad -v`

Expected: FAIL because package files are not implemented yet.

- [ ] **Step 3: Implement minimal config loader**

Use `gopkg.in/yaml.v3`. Define structs for sources and queries. Return clear errors for missing file, duplicate source name, empty URL, empty query value, and unsupported query kind.

- [ ] **Step 4: Implement CLI skeleton**

`main.go` must parse the first arg as the subcommand and print a useful usage message for unknown subcommands. The first implementation may return `not implemented` for all real work subcommands.

- [ ] **Step 5: Run tests**

Run: `go test ./irrindex/internal/config ./irrindex/cmd/irrindex -v`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod irrindex
git commit -m "feat: add irrindex config skeleton"
```

## Task 2: RPSL Parser

**Files:**
- Create: `irrindex/internal/rpsl/parser.go`
- Create: `irrindex/internal/rpsl/parser_test.go`
- Create: `irrindex/testdata/rpsl/ripe-route.txt`
- Create: `irrindex/testdata/rpsl/route6-multi-mnt.txt`
- Create: `docs/research/2026-06-22-rpsl-parser-spike.md`

**Interfaces:**
- Consumes: source objects from `config.SourcesConfig`
- Produces: `rpsl.Object{Type string, Prefix string, StartIP string, EndIP string, Attrs map[string][]string}`
- Produces: `rpsl.Parse(reader io.Reader) ([]Object, error)`

- [ ] **Step 1: Run parser dependency spike**

Compare these options on 20-object samples from RIPE, RADb, ARIN IRR, APNIC, and LACNIC:

- minimal Go block parser
- `github.com/frederic-arr/rpsl-go`
- IRRd parser behavior through a small local IRRd fixture if available

Write the result to `docs/research/2026-06-22-rpsl-parser-spike.md` with a recommendation. The default recommendation remains the minimal Go parser only if it correctly preserves repeated attributes, continuation lines, route/route6 prefixes, origin ASNs, `mnt-by`, inetnum ranges, and invalid/non-UTF-8 text handling for all useful samples. If a public source lacks `route`/`mnt-by` fields, document that as a source-suitability issue rather than a parser failure.

- [ ] **Step 2: Write parser tests**

Cover these fixture cases:

```text
route:          193.254.30.0/24
origin:         AS12726
mnt-by:         AS12312-MNT
mnt-by:         DE-COLT-MNT
source:         RIPE
```

```text
route6:         2001:db8::/32
origin:         AS65536
mnt-by:         FIRST-MNT
mnt-by:         SECOND-MNT
source:         TEST
```

Assert object type, prefix, origin, both maintainer values, and source.

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./irrindex/internal/rpsl -v`

Expected: FAIL because parser is missing.

- [ ] **Step 4: Implement block parser**

Read line by line, split objects on blank lines, ignore comment lines beginning with `%` or `#`, parse `key: value`, preserve repeated keys, and support continuation lines by appending to the previous key value with a space.

- [ ] **Step 5: Run parser tests**

Run: `go test ./irrindex/internal/rpsl -v`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add irrindex/internal/rpsl irrindex/testdata/rpsl docs/research/2026-06-22-rpsl-parser-spike.md
git commit -m "feat: parse RPSL route objects"
```

## Task 2A: IRRd Feasibility Spike

**Files:**
- Create: `docs/research/2026-06-22-irrd-spike.md`
- Optional create: `irrindex/experiments/irrd/docker-compose.yml`

**Interfaces:**
- Produces: recommendation on whether IRRd should replace, complement, or only validate the custom Go/SQLite path

- [ ] **Step 1: Run a local IRRd experiment**

Use a minimal IRRd setup with PostgreSQL and Redis. Import a small fixture containing route, route6, and multi-`mnt-by` objects.

- [ ] **Step 2: Verify current query equivalents**

Run these query classes against IRRd:

```bash
whois -h 127.0.0.1 -p 43 -- "-i origin AS16625"
whois -h 127.0.0.1 -p 43 -- "-i mnt-by DATACAMP-MNT"
curl "http://127.0.0.1:8080/v1/whois/?q=-i%20origin%20AS16625"
```

Also test GraphQL `asnPrefixes` and `databaseStatus`.

- [ ] **Step 3: Record operational cost**

In `docs/research/2026-06-22-irrd-spike.md`, record setup commands, import time, DB size, RAM use, query behavior, source filtering behavior, and whether GitHub Actions can realistically run it as a shadow job.

- [ ] **Step 4: Decide**

Choose one:

- Use IRRd as the local query backend and reduce custom Go code to comparison/reporting.
- Keep Go/SQLite as planned and use IRRd only as a parser/query reference.
- Defer IRRd entirely because operational cost is too high for this repo.

- [ ] **Step 5: Commit**

```bash
git add docs/research/2026-06-22-irrd-spike.md irrindex/experiments/irrd
git commit -m "docs: evaluate IRRd for local IRR shadowing"
```

## Task 2B: Ready-Made BGP Dataset Spike

**Files:**
- Create: `docs/research/2026-06-22-bgp-datasets.md`

**Interfaces:**
- Produces: decision on whether to add CAIDA pfx2as before MRT parsing

- [ ] **Step 1: Compare observed-BGP sources**

Evaluate:

- RIPE RISWhois dumps
- CAIDA RouteViews prefix-to-AS daily files
- BGPKIT Parser CLI against one RouteViews RIB
- BGPStream/BGPReader for the same time window

- [ ] **Step 2: Test sample ASNs**

For `AS714`, `AS16625`, `AS174`, `AS32934`, and `AS9009`, compare prefix counts and notable differences across the observed-BGP sources.

- [ ] **Step 3: Record recommendation**

In `docs/research/2026-06-22-bgp-datasets.md`, recommend one source for the first observed-BGP table and list deferred sources.

- [ ] **Step 4: Commit**

```bash
git add docs/research/2026-06-22-bgp-datasets.md
git commit -m "docs: evaluate ready-made BGP origin datasets"
```

## Task 2C: Go vs Rust Toolchain Spike

**Files:**
- Create: `docs/research/2026-06-22-go-vs-rust.md`

**Interfaces:**
- Produces: language recommendation for `irrindex` before implementation starts

- [ ] **Step 1: Define project-specific criteria**

Evaluate Go and Rust against these criteria:

- CI setup cost on GitHub Actions.
- Static binary distribution.
- Streaming gzip/HTTP parsing ergonomics.
- SQLite support and migration ergonomics.
- RPSL parsing ecosystem.
- BGP/MRT ecosystem, including BGPKIT Parser.
- Developer velocity for this repo.
- Memory safety and parser correctness risk.
- Ease of producing a small `whois-shim` CLI.
- Long-term maintenance by the user.

- [ ] **Step 2: Research available libraries**

For Go, check:

- SQLite driver options.
- gzip/HTTP streaming in the standard library.
- available RPSL parser packages such as `rpsl-go`.
- BGP/MRT parser maturity.

For Rust, check:

- SQLite driver options.
- gzip/HTTP streaming crates.
- available RPSL parser crates.
- BGPKIT Parser integration and CLI reuse.

- [ ] **Step 3: Run no-install local capability check**

Without installing packages unless the user explicitly approves, record:

```bash
go version
rustc --version
cargo --version
command -v bgpkit-parser || true
command -v bgpreader || true
```

- [ ] **Step 4: Decide**

Choose one:

- Implement the main tool in Go and treat BGPKIT/BGPStream as optional external tools.
- Implement the main tool in Rust to reuse BGPKIT Parser more directly.
- Split the project: Go/SQLite for IRR/RPSL and external Rust/BGPKIT only for deferred MRT parsing.

- [ ] **Step 5: Commit**

```bash
git add docs/research/2026-06-22-go-vs-rust.md docs/bulk-db-augment-plan.md
git commit -m "docs: add Go vs Rust toolchain research task"
```

## Task 3: RISWhois Parser

**Files:**
- Create: `irrindex/internal/ris/parser.go`
- Create: `irrindex/internal/ris/parser_test.go`
- Create: `irrindex/testdata/ris/riswhoisdump-ipv4.txt`

**Interfaces:**
- Produces: `ris.Record{ASN string, Prefix string, PeerCount int}`
- Produces: `ris.Parse(reader io.Reader) ([]Record, error)`

- [ ] **Step 1: Write parser tests**

Use this fixture:

```text
%
% header
%
64073	0.0.0.0/0	1
16625	23.32.0.0/11	12
714	17.0.0.0/8	25
```

Assert that comment lines are skipped, `0.0.0.0/0` is flagged as a default route, and normal records parse with `AS` prefix normalized to `AS16625` and `AS714`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./irrindex/internal/ris -v`

Expected: FAIL because parser is missing.

- [ ] **Step 3: Implement parser**

Parse whitespace-delimited rows as `asn prefix peer_count`. Normalize ASN to `AS<number>`. Return records in input order.

- [ ] **Step 4: Run parser tests**

Run: `go test ./irrindex/internal/ris -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add irrindex/internal/ris irrindex/testdata/ris
git commit -m "feat: parse RISWhois dumps"
```

## Task 4: SQLite Store And Query Layer

**Files:**
- Create: `irrindex/internal/store/schema.sql`
- Create: `irrindex/internal/store/store.go`
- Create: `irrindex/internal/store/store_test.go`
- Create: `irrindex/internal/query/query.go`
- Create: `irrindex/internal/query/query_test.go`

**Interfaces:**
- Consumes: `rpsl.Object`, `ris.Record`
- Produces: `store.Open(path string) (*DB, error)`
- Produces: `(*DB).InsertRPSLSource(sourceName string, objects []rpsl.Object) error`
- Produces: `(*DB).InsertRISSource(sourceName string, records []ris.Record, family int) error`
- Produces: `query.ByOrigin(db *store.DB, asn string, family int, sources []string) ([]string, error)`
- Produces: `query.ByMaintainer(db *store.DB, mnt string, family int, sources []string) ([]string, error)`

- [ ] **Step 1: Write store/query tests**

Build a temporary SQLite DB from synthetic RPSL and RIS records. Assert:

- `ByOrigin(AS16625, 4, nil)` returns the RPSL route and RIS prefix.
- `ByMaintainer(FIRST-MNT, 6, nil)` returns the IPv6 route.
- multi-valued `mnt-by` creates results for both maintainers.
- `0.0.0.0/0` is not returned.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./irrindex/internal/store ./irrindex/internal/query -v`

Expected: FAIL because store and query layer are missing.

- [ ] **Step 3: Implement schema and insert logic**

Use transactions. Batch inserts in groups of at least 10,000 rows. Normalize prefixes through Go's `net/netip` where possible. Reject default routes before insertion and increment a dropped-default counter.

- [ ] **Step 4: Implement query logic**

Return stable sorted unique CIDRs. Treat unknown ASN or maintainer as empty result with nil error.

- [ ] **Step 5: Run tests**

Run: `go test ./irrindex/internal/store ./irrindex/internal/query -v`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add irrindex/internal/store irrindex/internal/query
git commit -m "feat: index and query routing snapshots"
```

## Task 5: Fetch And Build Commands

**Files:**
- Create: `irrindex/internal/fetch/fetch.go`
- Create: `irrindex/internal/fetch/fetch_test.go`
- Modify: `irrindex/cmd/irrindex/main.go`

**Interfaces:**
- Consumes: `config.SourcesConfig`
- Produces: `fetch.FetchAll(cfg SourcesConfig, cacheDir string) (Manifest, error)`
- Produces: CLI `fetch` and `build`

- [ ] **Step 1: Write fetch tests with local HTTP server**

Serve small gzip fixtures from `httptest.Server`. Assert content is downloaded once, SHA256 is recorded, failed source records an error, and other sources continue.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./irrindex/internal/fetch -v`

Expected: FAIL because fetcher is missing.

- [ ] **Step 3: Implement streaming fetcher**

Use HTTP(S) GET with retry count 3 and timeout 60 seconds per source. Write compressed bytes to cache. Do not decompress to disk.

- [ ] **Step 4: Implement build command**

Open each cached gzip, stream into the relevant parser, insert into SQLite, and print row counts by source. If one non-critical source fails, write `ok=0` in `sources` and continue. If all sources fail, exit non-zero.

- [ ] **Step 5: Run tests**

Run: `go test ./irrindex/internal/fetch ./irrindex/internal/store ./irrindex/internal/query -v`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add irrindex/internal/fetch irrindex/cmd/irrindex
git commit -m "feat: fetch and build local routing index"
```

## Task 6: Query And Whois-Shim Commands

**Files:**
- Modify: `irrindex/cmd/irrindex/main.go`
- Create: `irrindex/cmd/irrindex/main_test.go`

**Interfaces:**
- Consumes: `query.ByOrigin`, `query.ByMaintainer`
- Produces: CLI `query origin`, `query mnt-by`, and `whois-shim`

- [ ] **Step 1: Write CLI tests**

Use a temporary DB fixture. Assert:

```bash
irrindex query origin AS16625 --db test.db --family 4
irrindex query mnt-by DATACAMP-MNT --db test.db --family 6
irrindex whois-shim --db test.db -h whois.radb.net -- "-i origin AS16625"
irrindex whois-shim --db test.db -h whois.ripe.net -- "-i mnt-by DATACAMP-MNT"
```

All commands must exit `0`; empty results must also exit `0`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./irrindex/cmd/irrindex -v`

Expected: FAIL because CLI commands are incomplete.

- [ ] **Step 3: Implement CLI commands**

Map `-h whois.radb.net` to source filter `radb`, `-h rr.ntt.net` to a configurable source group, `-h whois.ripe.net` to `ripe`, `-h riswhois.ripe.net` to `ris`, and unknown hosts to no source filter with a warning.

- [ ] **Step 4: Run CLI tests**

Run: `go test ./irrindex/cmd/irrindex -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add irrindex/cmd/irrindex
git commit -m "feat: add local query and whois shim commands"
```

## Task 7: Query Manifest

**Files:**
- Modify: `irrindex/config/queries.yaml`
- Create: `irrindex/internal/manifest/extract.go`
- Create: `irrindex/internal/manifest/extract_test.go`

**Interfaces:**
- Produces: `manifest.ExtractQueries(repoRoot string) ([]QuerySpec, error)`
- Produces: generated `queries.yaml` entries for current downloader queries

- [ ] **Step 1: Write extraction tests**

Use sample bash snippets with:

```bash
get_routes 'AS16625'
for asn in AS174 AS1239; do get_routes "$asn"; done
get_maintained 'DATACAMP-MNT'
```

Assert extracted service, kind, value, and host list from helper definitions.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./irrindex/internal/manifest -v`

Expected: FAIL because extractor is missing.

- [ ] **Step 3: Implement conservative extractor**

Use regex-based extraction for current patterns only. If a script has dynamic shell that cannot be parsed, emit a warning and require a manual `queries.yaml` entry.

- [ ] **Step 4: Generate current query manifest**

Run:

```bash
go run ./irrindex/cmd/irrindex extract-queries --repo . --out irrindex/config/queries.yaml
```

Expected: `queries.yaml` includes 26 origin-AS services and 20 maintainer services from the current repo.

- [ ] **Step 5: Commit**

```bash
git add irrindex/config/queries.yaml irrindex/internal/manifest
git commit -m "feat: capture downloader query manifest"
```

## Task 8: Compare Reports

**Files:**
- Create: `irrindex/internal/compare/compare.go`
- Create: `irrindex/internal/compare/compare_test.go`
- Modify: `irrindex/cmd/irrindex/main.go`

**Interfaces:**
- Consumes: `queries.yaml`, SQLite DB, current `*/ipv4.txt` and `*/ipv6.txt`
- Produces: `comparison.md`
- Produces: `comparison.json`

- [ ] **Step 1: Write comparison tests**

Use a fixture where live has `10.0.0.0/24`, local has `10.0.0.0/24` and `10.0.1.0/24`. Assert JSON fields:

```json
{
  "status": "local_includes_live",
  "live_count": 1,
  "local_count": 2,
  "local_only_count": 1,
  "live_only_count": 0
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./irrindex/internal/compare -v`

Expected: FAIL because comparison is missing.

- [ ] **Step 3: Implement comparison engine**

Normalize CIDRs exactly enough for set comparison. Do not mutate service outputs. Write markdown tables sorted by worst `live_only_count` first.

- [ ] **Step 4: Run comparison tests**

Run: `go test ./irrindex/internal/compare -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add irrindex/internal/compare irrindex/cmd/irrindex
git commit -m "feat: compare local index coverage"
```

## Task 9: Shadow Workflow

**Files:**
- Create: `.github/workflows/irr-snapshot.yml`
- Modify: `.gitignore`

**Interfaces:**
- Consumes: `irrindex` binary and existing committed downloader outputs
- Produces: GitHub Actions artifacts:
  - `irrindex.db`
  - `source-manifest.json`
  - `comparison.md`
  - `comparison.json`

- [ ] **Step 1: Add ignored generated paths**

Add:

```gitignore
.irr-cache/
snapshots/
indexes/
*.db
*.db-shm
*.db-wal
```

- [ ] **Step 2: Add workflow**

The workflow must:

```yaml
name: "IRR Snapshot Shadow"

on:
  workflow_dispatch:
  schedule:
    - cron: "38 5 * * *"

jobs:
  snapshot:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
      - name: Build irrindex
        run: go build -o ./irrindex-bin ./irrindex/cmd/irrindex
      - name: Fetch sources
        run: ./irrindex-bin fetch --config irrindex/config/sources.yaml --cache-dir "$RUNNER_TEMP/irr-cache" --manifest "$RUNNER_TEMP/source-manifest.json"
      - name: Build index
        run: ./irrindex-bin build --config irrindex/config/sources.yaml --cache-dir "$RUNNER_TEMP/irr-cache" --db "$RUNNER_TEMP/irrindex.db"
      - name: Verify index
        run: ./irrindex-bin verify --db "$RUNNER_TEMP/irrindex.db"
      - name: Compare with committed outputs
        run: ./irrindex-bin compare --db "$RUNNER_TEMP/irrindex.db" --queries irrindex/config/queries.yaml --live-root . --out "$RUNNER_TEMP/irr-report"
      - name: Add report to summary
        run: cat "$RUNNER_TEMP/irr-report/comparison.md" >> "$GITHUB_STEP_SUMMARY"
      - uses: actions/upload-artifact@v4
        with:
          name: irr-snapshot-${{ github.run_id }}
          path: |
            ${{ runner.temp }}/irrindex.db
            ${{ runner.temp }}/source-manifest.json
            ${{ runner.temp }}/irr-report/
```

- [ ] **Step 3: Run local workflow-equivalent smoke**

Run:

```bash
go build -o /tmp/irrindex ./irrindex/cmd/irrindex
/tmp/irrindex fetch --config irrindex/config/sources.yaml --cache-dir /tmp/irr-cache --manifest /tmp/source-manifest.json
/tmp/irrindex build --config irrindex/config/sources.yaml --cache-dir /tmp/irr-cache --db /tmp/irrindex.db
/tmp/irrindex verify --db /tmp/irrindex.db
```

Expected: commands complete or fail only for documented network/source issues; no service output files are changed.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/irr-snapshot.yml .gitignore
git commit -m "ci: add shadow IRR snapshot workflow"
```

## Task 10: Snapshot Retention And Manifests

**Files:**
- Create: `irrindex/internal/manifest/retention.go`
- Create: `irrindex/internal/manifest/retention_test.go`
- Modify: `irrindex/cmd/irrindex/main.go`

**Interfaces:**
- Produces: `irrindex manifest summarize --db ... --manifest ...`
- Produces: retention policy for artifacts and optional releases

- [ ] **Step 1: Write retention tests**

Given 20 daily snapshots, 10 weekly snapshots, and 8 monthly snapshots, assert retention keeps:

- last 14 daily
- last 8 weekly
- last 6 monthly

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./irrindex/internal/manifest -run Retention -v`

Expected: FAIL because retention logic is missing.

- [ ] **Step 3: Implement retention summary**

Do not delete anything in the first implementation. Only compute and print which artifacts would be retained or pruned.

- [ ] **Step 4: Commit**

```bash
git add irrindex/internal/manifest irrindex/cmd/irrindex
git commit -m "feat: summarize snapshot retention"
```

## Task 11: Augment Mode Pilot

**Files:**
- Create: `utils/db-augment.sh`
- Modify later, one at a time: `akamai/downloader.sh`, `apple/downloader.sh`, `vpnhosts/downloader.sh`

**Interfaces:**
- Produces: shell functions `db_routes ASN` and `db_mnt MNT`
- Consumes: `IRRINDEX_BIN` and `IRRINDEX_DB`

- [ ] **Step 1: Implement shell helper after shadow reports are stable**

Create:

```bash
db_routes() {
  if [ -n "${IRRINDEX_BIN:-}" ] && [ -n "${IRRINDEX_DB:-}" ] && [ -s "${IRRINDEX_DB:-}" ]; then
    "$IRRINDEX_BIN" query origin "$1" --db "$IRRINDEX_DB" || true
  fi
}

db_mnt() {
  if [ -n "${IRRINDEX_BIN:-}" ] && [ -n "${IRRINDEX_DB:-}" ] && [ -s "${IRRINDEX_DB:-}" ]; then
    "$IRRINDEX_BIN" query mnt-by "$1" --db "$IRRINDEX_DB" || true
  fi
}
```

- [ ] **Step 2: Pilot only after evidence**

Pilot candidates:

- `akamai`: large origin-AS fanout.
- `apple`: mixed maintainer, inetnum, ARIN org, and origin-AS.
- `vpnhosts`: worst maintainer-heavy case.

Do not edit these scripts until the shadow workflow has at least 14 consecutive runs and report review says local results are useful.

- [ ] **Step 3: Apply augment patch one downloader at a time**

For a pilot downloader, source `utils/db-augment.sh`, append `db_routes` or `db_mnt` after the current live whois commands, and keep all existing live whois commands.

- [ ] **Step 4: Run downloader locally**

Run:

```bash
IRRINDEX_BIN=/tmp/irrindex IRRINDEX_DB=/tmp/irrindex.db bash akamai/downloader.sh
python utils/merge.py --source=akamai/ipv4.txt | sort -h > /tmp/akamai-ipv4-merged-test.txt
```

Expected: downloader exits successfully and no `/0` appears.

- [ ] **Step 5: Commit pilot separately**

```bash
git add utils/db-augment.sh akamai/downloader.sh
git commit -m "feat: augment akamai with local IRR index"
```

## Migration Rules

Use these exact gates:

- Shadow mode can start as soon as `irr-snapshot.yml` exists and does not touch production outputs.
- Augment mode for a downloader requires at least 14 successful shadow reports.
- Augment mode must be additive: current whois output remains present, local DB results are unioned after it.
- Cutover from one live mirror to local lookup requires at least 21 consecutive clean days for that downloader/query/mirror.
- A clean day means local output covers live output after CIDR normalization and no source row-count floor was violated.
- Do not remove the final live whois canary for a query until the user explicitly approves that later cleanup.

## Safety Gates

- Source row floor: fail `verify` if a source's route row count drops more than 40% versus previous manifest, unless the source itself failed and previous index is being reused.
- Whole-index floor: fail `verify` if total route rows drop more than 30% versus previous manifest.
- Default-route guard: drop and report all `/0` records.
- Staleness guard: warn if any source has `Last-Modified` older than 72 hours.
- Network failure policy: one source may fail; all sources failing is a hard failure.
- Comparison policy: comparison gaps are warnings in shadow mode, not CI failures.
- Production update policy: existing `update.yml` remains the only production writer until an explicit augment task is reviewed.

## Verification Commands

Run before marking the implementation branch ready for review:

```bash
go test ./...
go build -o /tmp/irrindex ./irrindex/cmd/irrindex
/tmp/irrindex query origin AS16625 --db /tmp/irrindex.db --family 4
/tmp/irrindex query mnt-by DATACAMP-MNT --db /tmp/irrindex.db --family 4
git status --short
```

Expected:

- Go tests pass.
- Binary builds.
- Query commands return CIDRs or empty output with exit code `0`.
- No service output files are modified in shadow mode.

## Open Decisions

- Decide whether shadow reports should be committed to a separate `irr-reports` branch or kept only as artifacts.
- Decide whether to archive weekly SQLite snapshots as GitHub Release assets after the first two weeks.
- Decide whether AFRINIC is included after checking the current access and use terms.
- Decide whether `whois-shim` or explicit `db_routes`/`db_mnt` helper calls are preferred for final cutover. The plan implements both but uses helper calls for augment mode because they are easier to review.

## Final Recommendation

Proceed with the local DB project, but keep it isolated until evidence exists. The right first deliverable is not changed IP lists; it is a repeatable shadow index, per-source provenance, and reports that show where the local DB is already a superset, where it is incomplete, and where it adds useful prefixes.
