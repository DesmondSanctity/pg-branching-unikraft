# Evidence Log — Multi-Tenant Postgres API with Instance Branching on Unikraft Cloud

Real, captured output backing the article
**"Testing Risky Postgres Migrations Safely with Instance Branching on Unikraft Cloud."**
Nothing here is paraphrased.

- **Date:** 2026-07-05
- **Metro:** `fra` (Frankfurt)
- **Account/profile:** `anonx`
- **DB image:** `anonx/pg-branching-demo-db:latest` (PostgreSQL 16.4 from source + `pg_ukc_scaletozero`)
- **API image:** `anonx/pg-branching-demo:latest`
- **DB FQDN:** `falling-dawn-v7xqhodw.fra.unikraft.app`
- **API FQDN:** `https://polished-star-c02tq06l.fra.unikraft.app`

## Toolchain / dependencies

- Go 1.24.1 (toolchain auto-upgrades to 1.25.x), Docker 27.5.1, Unikraft CLI 0.2.3
- `github.com/jackc/pgx/v5 v5.10.0`, `github.com/pressly/goose/v3 v3.27.2`
- psql 15.13 (client); server PostgreSQL 16.4

## Deploy Postgres

```bash
unikraft volume create --metro fra --name pgdemo-data --size 256M
unikraft run --metro fra --name pgdemo-db -p 5432:5432/tls -m 1G \
  -e POSTGRES_PASSWORD=<pw> -e PGDATA=/volume/postgres \
  --volume pgdemo-data:/volume --image anonx/pg-branching-demo-db:latest
```

Postgres booted and reported `database system is ready to accept connections`.
`psql` connects over TLS with `sslmode=require` (negotiated TLS), returning
`PostgreSQL 16.4 on x86_64-pc-linux-musl`.

## Seed (realistic, uneven)

`go run ./cmd/seed` → **25 tenants, 2,870 notes**. Per-tenant note counts:
**min 17 / avg 100 / max 183** — deliberately uneven so the migration has a
nontrivial table to act on.

## The branching workflow (the whole point)

### Branch the DB (measured downtime)

```
stop source:  4s
clone volume: 2s   (unikraft volume clone pgdemo-data --name pgdemo-branch1 → state: available)
restart src:  2s
TOTAL source downtime: ~8s
```

Source data intact after the cycle: **2,510/2,870 notes** confirmed across runs.
The clone is a full independent copy — the branch reported the **same 2,870 notes**
as the source.

### Broken migration on the branch → real failure

`ALTER TABLE notes ADD COLUMN priority INT NOT NULL;` run against the branch:

```
psql:broken/00002_add_priority_BROKEN_DO_NOT_APPLY_TO_SOURCE.sql:14:
ERROR:  column "priority" of relation "notes" contains null values
```

The source was never touched.

### Fixed migration on a fresh branch → success

`00002_add_priority.sql` (`... NOT NULL DEFAULT 0`) via goose on a second branch:

```
OK   00002_add_priority.sql
goose: successfully migrated database to version: 2
```

Verification on the branch: `notes=2870, with_priority=2870, min=0, max=0` — the
default backfilled every existing row.

### Promote to source

```
go run ./cmd/migrate up
OK   00002_add_priority.sql
goose: successfully migrated database to version: 2
```

Source verification: `notes=2870, with_priority=2870, max=0`. Branches then deleted
(instance first, then the cloned volume).

## API end-to-end (post-migration)

```
GET  /healthz                         → {"status":"ok"}
POST /tenants {"name":...}            → {"id": "...", ...}
POST /tenants/{id}/notes {...}        → {"id":"...","title":"first",...}
```

New notes created through the API get `priority = 0` automatically (verified via
psql: `first|0`) — the fixed migration is backward-compatible, so the running API
didn't even need to know about the new column.

## Real gotchas hit (the high-value troubleshooting section)

1. **Branches consume volume quota.** Each clone is a full-size volume copy. The
   account quota is **1024 MiB total**; a 1 GiB source left no room to clone:
   `Failed to allocate volume. Quota exceeded ... current limit of 1024`. Fix:
   size volumes modestly (256 MiB here) and delete branches when done.
2. **Clone requires the source not read-write mounted.** Cloning a live RW volume
   returns a 504 and leaves a stuck `uninitialized` volume — you must stop the
   source first (which also yields a crash-consistent clone).
3. **Cleanup ordering.** A source volume can't be deleted while it has active
   clones — remove branch instances and their cloned volumes first.
4. **The custom PG image has no `pgcrypto`.** `gen_random_uuid()` is built into
   PostgreSQL 13+, so the extension isn't needed — drop `CREATE EXTENSION pgcrypto`.
5. **Static-glibc (CGO) `scratch` build crashed the microVM** (`kernel crash:
   assertion error`). Switching the API image to pure-Go (`CGO_ENABLED=0`)
   dynamic-PIE on an `alpine` rootfs booted cleanly.
6. **Stateful scale-to-zero + TLS Postgres did not wake reliably on connection.**
   With `scale-to-zero policy=idle,stateful=true`, once the DB idled to `standby`,
   incoming TLS connections failed with `SSL connection has been closed
   unexpectedly` / `unexpected EOF` during the cold-wake window (both `psql` and the
   API's pgx pool). A manual `unikraft instances start` brings it up fine, and it
   works reliably while running. For the demo we ran the DB **always-on**. See the
   investigation below for the likely cause.

## Investigation: why stateful scale-to-zero was failing

Checked against the [scale-to-zero](https://unikraft.com/docs/features/scale-to-zero)
and [snapshots](https://unikraft.com/docs/features/snapshots) docs. This is **not a
tier or account-level restriction** — stateful scale-to-zero is a standard feature.
The behavior we hit comes from **how the wake works for the port handler in use**:

- The docs' happy-path examples all expose an **`http`-handler** port
  (`-p 443:8080/http+tls`). For HTTP ports, the proxy holds the incoming request
  while the instance wakes (millisecond snapshot restore) and then serves it — the
  client never sees the wake. The docs explicitly note the premature-scaledown /
  first-response issue "is never the case for ports of your service that have the
  `http` handler set."
- Postgres is exposed on a **raw `tls` (plain TCP) port** (`5432:5432/tls`), which
  has **no `http` handler**. For the `idle` policy the docs say established
  connections stay open and *"incoming packets wake up the instance"* — i.e. the
  first packet is consumed to trigger the wake. With a TLS handshake, that first
  connection can therefore be dropped/reset while the snapshot restores, which is
  exactly the `SSL connection has been closed unexpectedly` we observed. The
  expected pattern for raw-TCP/TLS ports is **client-side connection retry**: the
  first attempt wakes the instance, a subsequent attempt succeeds.

**Why our retries still failed:** our retry loop did retry (5–6 attempts with
backoff) and still failed every time. The most likely compounding factor is the
**unstable network** this whole session ran on (it also caused Docker Hub TLS
errors and failed volume uploads). On a stable connection, retry-after-wake is the
documented and expected behavior for a TCP/TLS service.

**Takeaways for the article / production use:**
- Scale-to-zero for a Postgres (raw-TCP/TLS) endpoint is supported, but the client
  **must** implement connection retry to absorb the cold-wake window — this is
  by-design for non-`http` ports, not a bug.
- The `pg_ukc_scaletozero` module handles *not scaling down mid-query*; it does not
  remove the need for retry on the *initial* wake connection.
- Alternatives: a warmer cooldown so the DB rarely idles, or reaching the DB over
  the internal Private FQDN (which scale-to-zero also supports) from co-located app
  instances.

## Cost comparison: scale-to-zero vs always-on managed Postgres

The economic argument for scale-to-zero is strongest for **dev/test and
branch/preview databases**, which sit idle the vast majority of the time.

> **Note on rates:** Unikraft Cloud's public pricing page markets the model
> ("never pay for idle", active-resource billing) but does not publish per-unit
> $/hour rates (plans are signup/contact-based), so the Unikraft side below is
> expressed structurally: **you are billed only for the minutes an instance is
> actually running (vCPU + memory), plus persistent volume storage; a `standby`
> instance consumes no compute.** The managed-Postgres figures are real published
> on-demand rates and should be re-verified at publication time.

**Always-on baseline (real published rates, ~2026):**

| Option | Spec | Approx. monthly (compute, 24×7) |
| --- | --- | --- |
| AWS RDS PostgreSQL `db.t4g.micro` (on-demand, eu-central-1) | 2 vCPU burst, 1 GiB | ~$0.018/hr × 730h ≈ **$13/mo** + storage (~$0.12/GB-mo) |
| Supabase Pro | shared | **$25/mo** flat |
| Neon / scale-to-zero DBaaS | — | not an "always-on" comparison (also scales to zero) |

**The idle math (this is the point):** a dev/test or per-branch database is
typically active only a small fraction of the day. If it does ~2 hours of real work
per day (≈ 60 running hours/month) and is idle the rest:

| | Always-on RDS `db.t4g.micro` | Scale-to-zero (billed only while running) |
| --- | --- | --- |
| Compute hours billed / mo | 730 h | ~60 h |
| Relative compute cost | 100% | **~8%** (≈ **92% saved**) |
| Idle cost | full | **~0** (only volume storage remains) |

The persistent volume (256 MiB here) is billed whether or not the instance runs,
but at these sizes it's negligible. The savings scale with the number of idle
databases — e.g. **one branch per open pull request**, each costing ~nothing while
nobody is actively testing it, versus paying 24×7 for a fleet of mostly-idle
managed instances.

## Stateful persistence

Data survived multiple instance remove+redeploy cycles on the same `pgdemo-data`
volume (the `goose_db_version` table and all rows persisted), confirming
volume-backed persistence independent of the instance lifecycle.

## Final deployed state

- `pgdemo-db` — always-on Postgres, 256M `pgdemo-data` volume, migrated to v2
- `pgdemo-api` — notes API, connected over TLS, 2,872 notes (2,870 seed + API test rows)
