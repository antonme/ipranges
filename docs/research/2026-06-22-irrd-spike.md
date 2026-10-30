# IRRd Feasibility Spike

Date: 2026-06-22
Branch: `bulk-db-augment-plan`

## Goal

Evaluate whether IRRd v4 should replace or reduce the planned custom Go/SQLite IRR indexer for the `ipranges` bulk-DB augmentation project.

The specific question: can IRRd serve as the local routing-registry backend for current downloader patterns such as:

```bash
whois -h whois.radb.net -- "-i origin AS16625"
whois -h whois.ripe.net -- "-i mnt-by DATACAMP-MNT"
curl "http://127.0.0.1:8080/v1/whois/?q=-i%20origin%20AS16625"
```

## Summary

IRRd is technically a strong fit for the IRR portion of the project, but it is not a drop-in replacement for the full proposed `irrindex` tool.

Recommendation: **keep the Go/SQLite plan as the default implementation, and use IRRd as a validation/reference backend first.** If a later CI spike proves that a containerized IRRd + PostgreSQL + Redis stack is cheap enough to run in GitHub Actions, we can revisit using IRRd as the local query backend and shrink the custom Go code to fetch orchestration and comparison reports.

## What IRRd Already Solves

IRRd v4 is explicitly an IRR database server for RPSL data. Its documented features include:

- validating, cleaning, storing, and indexing RPSL data
- multiple query interfaces
- mirroring other IRR databases using file imports and NRTM
- providing NRTM mirroring and full exports to other databases

Source: <https://irrd.readthedocs.io/en/stable/>

This directly overlaps with the hardest custom work in the original plan: RPSL parsing, source imports, indexed route queries, and source status.

## Query Fit

IRRd supports both raw TCP whois and HTTPS whois queries.

Source: <https://irrd.readthedocs.io/en/stable/users/queries/whois/>

Relevant matches for this repo:

- Raw TCP queries on port 43 by default.
- HTTPS `/v1/whois/?q=...` queries.
- RIPE-style `-i <attribute> <value>` queries.
- `-i origin AS23456`, which maps to existing `get_routes` helpers.
- Multi-valued attributes are considered when using `-i`, which matters for repeated `mnt-by` fields.
- `-s <sources>` source filtering.
- Faster IRRd-style `!gAS<asn>` and `!6AS<asn>` origin-prefix queries.
- `!o<mntner-name>` searches objects with a maintainer in `mnt-by`.
- `!J` returns JSON source/mirroring status.

GraphQL is also available at `/graphql`, including:

- `databaseStatus`, with source freshness/status fields.
- `asnPrefixes`, which directly answers origin-AS prefix queries.

Source: <https://irrd.readthedocs.io/en/stable/users/queries/graphql/>

This is a good match for shadow reports and source freshness checks.

## Import Fit

IRRd has `irrd_load_database` and `irrd_update_database` commands.

Important documented behavior:

- `irrd_load_database --source TEST testv1.db` loads one source from a local file.
- Input file cannot be gzipped.
- The source must already exist in config.
- Successful load replaces all objects for that source.
- On first validation error, the import aborts without changing stored data.
- Manual file loading cannot be mixed with regular mirroring settings for the same source.
- `irrd_update_database` is not intended for larger data sets over 150,000 objects.

Source: <https://irrd.readthedocs.io/en/stable/users/mirroring/>

This matters for `ipranges`: daily full imports of RIPE/RADb-sized data probably mean `irrd_load_database`, not `irrd_update_database`, unless we use NRTM/mirror mode.

## Operational Cost

IRRd is materially heavier than the custom Go/SQLite artifact plan.

Documented requirements:

- CPython 3.10 through 3.14.
- PostgreSQL.
- Redis 5 or newer.
- Rust compiler may be needed for dependencies.
- Recommended baseline is 32 GB RAM, 4 CPU cores, and 150 GB SSD disk, though docs say smaller deployments may work depending on data volume/query load.
- PostgreSQL is the only source of IRRd data.
- The initial PostgreSQL DB is roughly three times the size of imported RPSL text.
- Redis must have `maxmemory` increased to 2 GB or unlimited; IRRd will exceed Redis defaults otherwise.

Source: <https://irrd.readthedocs.io/en/stable/admins/deployment/>

For this repo's first shadow workflow, those requirements are the main downside. A one-file SQLite artifact is easier to cache, upload, compare, and inspect in GitHub Actions.

## Local Experiment Attempt

Commands run:

```bash
git status --short --branch
docker --version
docker compose version
docker info --format '{{.ServerVersion}} {{.OSType}} {{.Architecture}}'
for p in python3.13 python3.12 python3.11; do command -v $p && $p --version; done
for c in postgres initdb pg_ctl createdb psql redis-server redis-cli; do echo "$c: $(command -v $c || echo missing)"; done
python3.11 -m venv /tmp/irrd-spike-venv
/tmp/irrd-spike-venv/bin/pip install --timeout 60 irrd==4.5.2
```

Observed local environment:

```text
branch: bulk-db-augment-plan
Docker CLI: Docker version 29.4.0, build 9d7ad9f
Docker Compose: Docker Compose version v5.1.2
Docker daemon: unavailable; configured OrbStack socket does not exist
PostgreSQL binaries: missing
Redis binaries: missing
Python: 3.11, 3.12, and 3.13 available
IRRd package: PyPI metadata/download timed out repeatedly; install was cancelled
```

Exact Docker daemon error:

```text
failed to connect to the docker API at unix:///Users/anton/.orbstack/run/docker.sock;
check if the path is correct and if the daemon is running:
dial unix /Users/anton/.orbstack/run/docker.sock: connect: no such file or directory
```

The full local IRRd query experiment was therefore not completed in this environment. The blocker is operational, not conceptual: no reachable Docker daemon, no local PostgreSQL/Redis, and a slow PyPI install path.

## What This Means For The Project

IRRd is a good candidate if we want a real local IRR server:

- It should cover `origin` and `mnt-by` IRR queries better than a fresh parser.
- It gives source status and query APIs we would otherwise implement.
- It can preserve whois-style semantics more faithfully than a first custom shim.

But IRRd does not cover everything:

- It does not replace RIPE RISWhois dumps or CAIDA/RouteViews observed-BGP tables.
- It does not replace ARIN Whois-RWS org-net lookup used by `utils/arin-org.py`.
- It does not generate repo-specific per-service comparison reports.
- It adds service dependencies: PostgreSQL and Redis.
- It is not a compact single-file snapshot artifact by default.

## Decision

For now: **keep Go/SQLite `irrindex` as the primary implementation path.**

Also add this constraint to the implementation plan:

- Before writing a full custom RPSL parser, run a second IRRd spike in an environment with working Docker or service containers.
- If IRRd can import a representative RIPE/RADb/ARIN/APNIC subset and answer `-i origin`, `-i mnt-by`, `!g`, `!6`, `!J`, GraphQL `asnPrefixes`, and GraphQL `databaseStatus` cheaply in CI, reconsider using IRRd as the local IRR backend.

## Recommended Next IRRd Test Environment

Run this on a machine or CI job with Docker daemon available:

```bash
docker compose version
```

Use services:

- `postgres:18-alpine` or `postgres:16-alpine`
- `redis:7-alpine`
- a Python 3.12 IRRd container built with `pip install irrd==4.5.2`

Use a tiny RPSL fixture:

```text
route:          203.0.113.0/24
origin:         AS16625
mnt-by:         DATACAMP-MNT
mnt-by:         SECOND-MNT
source:         TEST

route6:         2001:db8::/32
origin:         AS16625
mnt-by:         DATACAMP-MNT
source:         TEST
```

Then verify:

```bash
irrd_database_upgrade --config /etc/irrd.yaml
irrd_load_database --config /etc/irrd.yaml --source TEST /data/test.db
whois -h 127.0.0.1 -p 43 -- "-i origin AS16625"
whois -h 127.0.0.1 -p 43 -- "-i mnt-by DATACAMP-MNT"
printf '!gAS16625\n' | nc 127.0.0.1 43
printf '!6AS16625\n' | nc 127.0.0.1 43
curl "http://127.0.0.1:8080/v1/whois/?q=-i%20origin%20AS16625"
curl -s http://127.0.0.1:8080/graphql -H 'content-type: application/json' \
  --data '{"query":"query { databaseStatus { source authoritative lastUpdate } }"}'
curl -s http://127.0.0.1:8080/graphql -H 'content-type: application/json' \
  --data '{"query":"query { asnPrefixes(asns: [16625], ipVersion: 4) { prefixes } }"}'
```

Metrics to record:

- import time
- PostgreSQL data size after import
- Redis memory after import
- resident memory for IRRd processes
- response time for `-i origin`, `!g`, `-i mnt-by`, and GraphQL `asnPrefixes`
- behavior for source filtering

## Recommendation For Immediate Next Work

Do the **Ready-Made BGP Dataset Spike** next, because it does not require PostgreSQL/Redis and can materially reduce the observed-BGP portion of the custom work. In parallel, keep IRRd as a candidate for a later Docker-backed CI spike.
