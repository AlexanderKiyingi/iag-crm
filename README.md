# iag-crm

Commercial CRM microservice for the IAG platform — Go/Gin backend aligned with the Customer Tower prototype in `index.html`.

Registry: [`subrepos.json`](../../../subrepos.json) · Dev port: **4101** · Gateway: **`/api/v1/crm/v1`**

## Stack

- **Go 1.25** + **Gin**
- **Postgres** (`crm_*` tables, `svc_iag_crm` role)
- **Platform JWT** (`aud=iag.crm`) — same pattern as contract-management
- **Embedded UI** — `index.html` served at `GET /` and `GET /ui` when `SERVE_UI=true`
- **Next.js** via gateway + CORS (`ALLOWED_ORIGINS`)

## Quick start

```bash
cd services/commercial/crm
cp .env.example .env
go run .
```

With platform stack: `docker compose up crm` from `deploy/` (registers gateway route, auth audience, permissions).

## Next.js integration

Full guide: [docs/FRONTEND_INTEGRATION.md](docs/FRONTEND_INTEGRATION.md) (auth, bootstrap, permissions, route catalog, SSE, OAuth).

Copy [docs/frontend.env.example](docs/frontend.env.example) to your Next.js app as `.env.local`.

Obtain a user token from `POST /api/v1/authentication/oauth/token` (aud includes `iag.gateway`), then:

```ts
import { crmApi } from "./docs/crm-api";
const boot = await crmApi.bootstrap(accessToken);
```

`GET /v1/bootstrap` returns session, RBAC pages/modals (from `index.html` roles), page titles, and API prefix.

## API coverage (29 UI pages)

| Section | Routes |
|---------|--------|
| Platform | `/bootstrap`, `/auth/session`, `/permissions/*`, `/lookups/:kind`, `/notifications`, `/platform/status` |
| Audit | `/audit`, `/audit/:id` |
| Admin (staff) | `/admin/audit`, `/admin/audit-logs`, `/admin/monitoring/*`, `/admin/bridge/sync` |
| Command | `/overview`, `/pipeline/board`, `/accounts`, `/contacts`, `/leads` |
| Sales | `/deals`, `/deals/:id/won`, `/deals/forecast`, `/quotes`, `/quotes/:id/send`, `/quotes/:id/sign`, `/activities`, `/activities/stream` |
| Marketing OS | `/marketing/hub`, `/segments`, `/journeys`, `/personas`, `/events`, `/content/assets`, `/email/sends`, `/social/posts`, `/seo/keywords`, `/marketing/budget`, `/mqls`, `/brand-kit`, `/campaigns`, `/marketing/demand-gen` |
| Engagement | `/tickets`, `/loyalty/*`, `/loyalty/tier-rules` |
| DMS bridge | `/bridge/*`, `/bridge/pending-imports/:id/assign`, `/outlets`, `/outlets/:id/360`, `/export-customers`, `POST /exports/views/:page` |
| Intelligence | `/insights/summary`, `/insights/signals`, `/ai/*`, `POST /seo/audit` |

Cross-entity dropdowns in modals map to `GET /v1/lookups/{accounts|contacts|deals|segments|campaigns|journeys|personas|events|outlets|contentLib}`.

## Auth & microservices

- Gateway proxies `/api/v1/crm` → CRM `:4101` (Bearer forwarded)
- Permissions registered at boot (`crm.*` + `audit.*`) when `SERVICE_CLIENT_SECRET` is set
- Handler-level `RequirePerm` on every route; gateway policies for audit/admin/destructive ops
- Kafka events on `iag.commercial` when `EVENT_BUS_ENABLED=true` (deal updates, lead convert, bridge sync, tickets)
- DMS/ERP/LIMS integration: bridge streams + stub enrichment on `/outlets/:id/360`, `/export-customers` (extend with live DMS in phase 2)

## Deployment

### Docker (monorepo root context)

```bash
# from repo root
docker build -f services/commercial/crm/Dockerfile -t iag-crm .
```

Compose (`deploy/docker-compose.yml`) uses the same Dockerfile. Gateway waits for CRM health when `READY_PROBE_UPSTREAMS=true`.

### Production checklist

- `ENVIRONMENT=production` (enables strict RBAC — tokens must carry `permissions`)
- `SEED_ON_EMPTY=false`, `SERVICE_CLIENT_SECRET` (min 16 chars)
- `CONSUMER_ENABLED=true`, `EVENT_BUS_ENABLED=true`, upstream URLs (`FINANCE_API_URL`, `DMS_API_URL`, …)
- `JOURNEY_RUNNER_INTERVAL=30s` for journey step execution
- `GOOGLE_OAUTH_*` / `MICROSOFT_OAUTH_*` for email + calendar sync
- Explicit `ALLOWED_ORIGINS` (not `*`)
- See [`docs/PRODUCTION_CHECKLIST.md`](docs/PRODUCTION_CHECKLIST.md) and [`config/.env.production.example`](config/.env.production.example)

### Smoke test

```bash
CRM_SMOKE_TOKEN=<jwt> sh services/commercial/crm/scripts/smoke_test.sh
```

## Environment

See [`.env.example`](.env.example). Every request requires a Bearer JWT with `aud=iag.crm`. Production also requires `ALLOWED_ORIGINS` and `SERVICE_CLIENT_SECRET`.
