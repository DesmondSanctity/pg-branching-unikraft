# pg-branching-demo

A minimal but realistic multi-tenant notes API in Go, backed by a **scale-to-zero
PostgreSQL** instance on [Unikraft Cloud](https://unikraft.com) — built to
demonstrate **testing a risky database migration safely using instance branching**.

The idea: the classic migration that passes on an empty dev database and detonates
in production (adding a `NOT NULL` column with no default to a table that already
has rows). Instead of finding out the hard way, you **clone the database's volume**
into a disposable branch, run the scary migration there against a real copy of the
data, watch it fail (or succeed), and only then promote the fixed version to the
source — which was never at risk.

## How "database branching" works here

PostgreSQL's data lives on a Unikraft Cloud **volume**. To branch:

1. **Stop** the source instance (a volume can only be cloned when it isn't mounted
   read-write — this also gives a crash-consistent copy).
2. **Clone** the volume: `unikraft volume clone <src> --name <branch>`.
3. **Restart** the source (it's back serving; the clone finishes copying in the
   background).
4. **Boot a new Postgres instance** on the cloned volume — that's your branch.

Measured source downtime for the stop→clone→restart cycle: **~8 seconds**, and it's
dominated by the instance stop/start, not the data size (the clone is
snapshot/copy-on-write style). A branch is a **full-size volume copy**, so it counts
against your account's volume quota — size volumes modestly and delete branches when
done.

## Architecture

```
cmd/api      # the notes API server (deployed to Unikraft Cloud)
cmd/seed     # seeds realistic, uneven multi-tenant data (run from your machine)
cmd/migrate  # applies the embedded goose migrations to DATABASE_URL
internal/db  # pgx connection pool
internal/api # HTTP handlers + router (Go 1.22 method routing)
migrations/  # 00001_init.sql, 00002_add_priority.sql (the FIXED version that ships)
broken/      # 00002_..._BROKEN_...sql — the failing version, for branch testing only
```

## Schema

```sql
tenants(id uuid pk, name text, created_at timestamptz)
notes(id uuid pk, tenant_id uuid fk, title text, body text, created_at timestamptz)
```

## API endpoints

| Method | Path                         | Body                              | Description                |
| ------ | ---------------------------- | --------------------------------- | -------------------------- |
| `POST` | `/tenants`                   | `{"name": "..."}`                 | Create a tenant            |
| `POST` | `/tenants/{tenant_id}/notes` | `{"title": "...", "body": "..."}` | Create a note              |
| `GET`  | `/tenants/{tenant_id}/notes` | —                                 | List a tenant's notes      |
| `GET`  | `/healthz`                   | —                                 | 200 if the DB is reachable |

## Requirements

- Go 1.25+
- Docker with BuildKit
- The [Unikraft CLI](https://unikraft.com/docs) + a Unikraft Cloud account (`unikraft login`)
- `psql` for verification

## 1. Deploy PostgreSQL

Build the Postgres image from the official example
([`unikraft-cloud/examples/postgres`](https://github.com/unikraft-cloud/examples/tree/main/postgres),
PostgreSQL 16 + the `pg_ukc_scaletozero` module), then deploy it on a persistent volume:

```bash
# a small volume is enough and leaves quota room for branches
unikraft volume create --metro fra --name pgdemo-data --size 256M

unikraft run --metro fra --name pgdemo-db \
  -p 5432:5432/tls -m 1G \
  -e POSTGRES_PASSWORD=<password> \
  -e PGDATA=/volume/postgres \
  --volume pgdemo-data:/volume \
  --scale-to-zero policy=idle,cooldown-time=300000,stateful=true \
  --image <your-org>/pg-branching-demo-db:latest
```

`stateful=true` + the volume is what persists data across scale-to-zero.

> **Note:** an aggressive scale-to-zero cooldown can drop the very first
> connection while the instance cold-wakes. For setup/seeding, deploy without
> `--scale-to-zero` (or use a longer cooldown), then re-enable it.

## 2. Configure & seed

```bash
cp .env.example .env    # fill in POSTGRES_PASSWORD, PGHOST (the DB FQDN), DATABASE_URL
export $(grep -v '^#' .env | xargs)

go run ./cmd/migrate up      # create the schema (tenants, notes)
go run ./cmd/seed            # 25 tenants, 10–200 notes each (uneven), ~2.8k notes
```

`DATABASE_URL` uses `sslmode=require` because Unikraft Cloud exposes Postgres over TLS:

```
postgres://postgres:<password>@<db-fqdn>:5432/postgres?sslmode=require
```

## 3. Deploy the API

```bash
unikraft build . --output <your-org>/pg-branching-demo:latest
unikraft run --metro fra --name pgdemo-api \
  -p 443:8080/http+tls \
  -e DATABASE_URL="postgres://postgres:<password>@<db-fqdn>:5432/postgres?sslmode=require" \
  --image <your-org>/pg-branching-demo:latest

curl https://<api-fqdn>/healthz
```

## 4. The branching workflow (test a risky migration safely)

The risky change lives in [broken/](broken/00002_add_priority_BROKEN_DO_NOT_APPLY_TO_SOURCE.sql):
`ALTER TABLE notes ADD COLUMN priority INT NOT NULL;` — no default, so it fails on a
populated table. **Never run it against the source.**

```bash
# Branch: stop the source, clone its volume, restart the source
unikraft instances stop pgdemo-db
unikraft volume clone pgdemo-data --name pgdemo-branch
unikraft instances start pgdemo-db

# Boot a Postgres instance on the branch
unikraft run --metro fra --name pgdemo-branch-db \
  -p 5432:5432/tls -m 1G \
  -e POSTGRES_PASSWORD=<password> -e PGDATA=/volume/postgres \
  --volume pgdemo-branch:/volume \
  --image <your-org>/pg-branching-demo-db:latest

# Run the BROKEN migration against the branch — watch it fail for real:
psql -h <branch-fqdn> -U postgres -f broken/00002_add_priority_BROKEN_DO_NOT_APPLY_TO_SOURCE.sql
# ERROR:  column "priority" of relation "notes" contains null values
```

The fix is `migrations/00002_add_priority.sql` (`... NOT NULL DEFAULT 0`). Test it on
a **fresh** branch, confirm it succeeds and backfills existing rows, then promote it
to the source:

```bash
export DATABASE_URL="postgres://postgres:<password>@<db-fqdn>:5432/postgres?sslmode=require"
go run ./cmd/migrate up        # applies the fixed migration to the source
```

Finally, delete the branches (remove the instance first, then the cloned volume — a
source volume can't be deleted while it has active clones):

```bash
unikraft instances remove pgdemo-branch-db
unikraft volume remove pgdemo-branch
```

## Security

- The DB password and `DATABASE_URL` are passed via environment variables and never
  committed (`.env` is gitignored).
- The broken migration is quarantined in `broken/` and named to prevent accidental
  use against the source.
