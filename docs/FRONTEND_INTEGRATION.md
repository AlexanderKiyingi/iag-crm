# CRM Frontend Integration Guide

Comprehensive guide for connecting a **Next.js** app to the CRM backend.
Covers auth, bootstrap, permissions, the full route catalog, SSE activity
stream, OAuth integrations, and the reference TypeScript client.

For deployment see [PRODUCTION_CHECKLIST.md](./PRODUCTION_CHECKLIST.md).
Reference client: [crm-api.ts](./crm-api.ts).

---

## 1. Authentication

CRM runs in **platform Bearer+aud mode** (hard cutover — no gateway header
trust). Every request — except health probes and OAuth callbacks — requires:

```
Authorization: Bearer <jwt>
```

The JWT must carry `aud=iag.crm`. CRM verifies signatures locally against
the auth service's JWKS.

Production enables **strict RBAC**: tokens without a `permissions` array are
denied even when authenticated.

### Two-hop audience (gateway + CRM)

1. **Gateway** verifies `aud=iag.gateway`.
2. Gateway forwards `Authorization` verbatim.
3. **CRM** re-verifies with `aud=iag.crm`.

### Token acquisition

```
POST /api/v1/authentication/oauth/token
  grant_type=password | refresh_token
→ access_token, refresh_token
```

**Frontend responsibilities:**
- Keep `access_token` in memory; refresh ~1 minute before expiry.
- On 401, attempt refresh; on second 401, redirect to login.
- On 403, hide the UI control.

### Common 401 / 403 causes

1. Token expired.
2. `aud` missing `iag.crm` or `iag.gateway`.
3. Missing **`platform.access_crm`** at gateway.
4. Missing module permission (e.g. `accounts.read` for bootstrap).
5. Strict RBAC: empty `permissions` in JWT.

---

## 2. Base URLs

| Environment | API base |
|---|---|
| Local direct | `http://localhost:4101/v1` |
| Local via gateway | `http://localhost:8080/api/v1/crm/v1` |
| Production | `https://iag-api-gateway-production.up.railway.app/api/v1/crm/v1` |

**Always go through the gateway in non-local environments.**

### Required frontend env vars

Copy [frontend.env.example](./frontend.env.example) to your Next.js app as
`.env.local` (local) or platform secrets (production).

```env
# Local (via gateway)
NEXT_PUBLIC_CRM_API_URL=http://localhost:8080/api/v1/crm/v1
NEXT_PUBLIC_AUTH_API_URL=http://localhost:8080/api/v1/authentication
NEXT_PUBLIC_GATEWAY_ORIGIN=http://localhost:8080
```

```env
# Production (Railway, via gateway)
NEXT_PUBLIC_CRM_API_URL=https://iag-api-gateway-production.up.railway.app/api/v1/crm/v1
NEXT_PUBLIC_AUTH_API_URL=https://iag-api-gateway-production.up.railway.app/api/v1/authentication
NEXT_PUBLIC_GATEWAY_ORIGIN=https://iag-api-gateway-production.up.railway.app
```

### CORS

Set `ALLOWED_ORIGINS` to include your Next.js origin. Auth uses Bearer
header — no cookies required by CRM.

---

## 3. Permission Model

CRM uses **short verb codenames**: `{module}.{action}` across 26 modules.

### 3.1 Modules (examples)

| Module | Examples |
|---|---|
| `accounts` | `accounts.read`, `accounts.create`, `accounts.update`, `accounts.delete` |
| `contacts` | `contacts.read`, … |
| `leads` | `leads.read`, `leads.create`, … |
| `deals` | `deals.read`, `deals.update`, … |
| `quotes` | `quotes.read`, `quotes.create`, … |
| `campaigns` | `campaigns.read`, … |
| `journeys` | `journeys.read`, … |
| `admin` | `admin.read`, … |

**Full catalogue:** [internal/models/permissions.go](../internal/models/permissions.go)
(26 modules × create/read/update/delete).

### 3.2 Built-in roles (UI mapping)

Roles are derived from platform groups in JWT claims and mapped to page/modal
visibility in bootstrap:

| Platform group | CRM role | Typical access |
|---|---|---|
| `superadmin` / `admin` | `admin` | All modules |
| `staff` / `manager` | `manager` | Sales + marketing write |
| `user` | `sales_rep` | Own-pipeline scoping |
| default | `viewer` | Read-only |

**Role source:** [internal/models/rbac.go](../internal/models/rbac.go)

### 3.3 Gateway service gate

Every proxied CRM route also requires **`platform.access_crm`**.

Superusers bypass permission checks.

### 3.4 Permissions API

| Method | Path | Permission | Description |
|---|---|---|---|
| GET | `/permissions/catalog` | `accounts.read` | Module/action matrix |
| GET | `/permissions/builtin` | `accounts.read` | Role → pages/modals |
| GET | `/permissions/me` | `accounts.read` | Enhanced permission context |
| POST | `/permissions/check` | `accounts.read` | Batch `{keys:[]}` → `{allowed:{}}` |

---

## 4. Bootstrap (primary app entry)

`GET /v1/bootstrap` is the recommended **single fetch on app load** — returns
layout, RBAC pages/modals, and API prefix in one round-trip.

**Permission:** `accounts.read` (+ `platform.access_crm` at gateway)

**Response shape (abbreviated):**

```json
{
  "service": "crm",
  "api_prefix": "/api/v1/crm/v1",
  "session": {
    "email": "user@example.com",
    "role": "manager",
    "pages": ["overview", "accounts", "deals", "..."],
    "modals": ["create-account", "..."],
    "permissions": ["accounts.read", "deals.read", "..."]
  },
  "permissions": { "role": "manager", "canMutate": true, "isStaff": true },
  "page_titles": { "overview": "Command Tower", "..." },
  "roles": { "..." },
  "modules": ["crm", "dms", "erp", "lims", "wms", "scm"]
}
```

**Related session routes:**

| Method | Path | Purpose |
|---|---|---|
| GET | `/auth/session` | Session + permissions only (lighter than bootstrap) |
| GET | `/lookups/:kind` | Modal dropdown data |
| GET | `/search?q=` | Global search |
| GET | `/notifications` | In-app notifications |
| GET | `/platform/status` | Upstream health (staff) |

**Lookup kinds:** `accounts`, `contacts`, `deals`, `segments`, `campaigns`,
`journeys`, `personas`, `events`, `outlets`, `contentLib`

---

## 5. Endpoint Catalog

All routes prefixed with base URL (§2). Listed permission is the **minimum**
service check; gateway also requires `platform.access_crm`.

### 5.1 Public probes (no auth)

| Method | Path |
|---|---|
| GET | `/health`, `/healthz`, `/ready` |
| GET | `/v1/health`, `/v1/health/live`, `/v1/health/ready` |

OAuth callbacks (browser redirect from Google/Microsoft):

| Method | Path |
|---|---|
| GET | `/integrations/oauth/:provider/callback` |

### 5.2 Dashboard

| Method | Path | Permission |
|---|---|---|
| GET | `/overview` | `accounts.read` |
| GET | `/pipeline/board` | `deals.read` |
| GET | `/deals/forecast` | `deals.read` |

### 5.3 Sales CRM

| Resource | List | Get | Create | Patch | Delete |
|---|---|---|---|---|---|
| Accounts | `GET /accounts` | `GET /accounts/:id` | POST | PATCH | DELETE |
| Account 360 | — | `GET /accounts/:id/360` | — | — | — |
| Contacts | `GET /contacts` | `GET /contacts/:id` | POST | PATCH | DELETE |
| Leads | `GET /leads` | `GET /leads/:id` | POST | PATCH | DELETE |
| Deals | `GET /deals` | `GET /deals/:id` | POST | PATCH | DELETE |
| Quotes | `GET /quotes` | `GET /quotes/:id` | POST | PATCH | DELETE |

**Actions:**

| Method | Path | Permission |
|---|---|---|
| POST | `/leads/:id/convert` | `leads.update` |
| PATCH | `/deals/:id/stage` | `deals.update` |
| POST | `/deals/:id/won` | `deals.update` |
| POST | `/quotes/:id/send` | `quotes.update` |
| POST | `/quotes/:id/sign` | `quotes.update` |

**Sales rep scoping:** when role is `sales_rep`, list endpoints auto-filter
by `claims.Email` as owner.

### 5.4 Marketing

| Area | Key routes |
|---|---|
| Hub | `GET /marketing/hub`, `/marketing/demand-gen` |
| Segments | CRUD `/segments` |
| Journeys | CRUD `/journeys`, steps, enroll, activate |
| Personas | CRUD `/personas` |
| Events | CRUD `/events` |
| Content | CRUD `/content/assets` |
| Email | `/email/sends` |
| Social | `/social/posts` |
| SEO | `/seo/keywords`, `/seo/audit` |
| Budget | `/marketing/budget` |
| MQLs | `/mqls` |
| Brand | `/brand-kit` |
| Campaigns | CRUD `/campaigns` |

Permissions follow `{module}.read|create|update|delete` per resource.

### 5.5 Engagement

| Method | Path | Permission |
|---|---|---|
| GET/POST/PATCH/DELETE | `/activities` | `activities.*` |
| GET | `/activities/stream` | `activities.read` (SSE) |
| GET | `/exports/views/:page` | per page module |
| GET | `/exports/jobs/:id` | per page module |
| CRUD | `/tickets` | `tickets.*` |
| CRUD | `/loyalty/*` | `loyalty.*` |

### 5.6 Bridge / DMS integration

| Method | Path | Permission |
|---|---|---|
| GET | `/bridge/status` | `bridge.read` |
| POST | `/bridge/sync` | `bridge.update` |
| GET | `/bridge/streams` | `bridge.read` |
| GET | `/bridge/pending-imports` | `bridge.read` |
| CRUD | `/outlets` | `outlets.*` |
| GET | `/outlets/:id/360` | `outlets.read` |
| POST | `/export-customers` | `accounts.read` |

### 5.7 Intelligence & AI

| Method | Path | Permission |
|---|---|---|
| GET | `/insights/summary` | `insights.read` |
| GET | `/insights/signals` | `insights.read` |
| GET | `/ai/suggestions` | `ai.read` |
| POST | `/ai/copilot/chat` | `ai.update` |

### 5.8 Audit

| Method | Path | Permission |
|---|---|---|
| GET | `/audit` | `audit.read` |
| GET | `/audit/:id` | `audit.read` |
| POST | `/audit` | `audit.create` |

### 5.9 Integrations (OAuth)

| Method | Path | Notes |
|---|---|---|
| GET | `/integrations/status` | Connection health |
| GET/POST/PATCH/DELETE | `/integrations/connections` | CRM connections |
| GET | `/integrations/oauth/:provider/start` | Start OAuth (authenticated) |
| GET | `/integrations/oauth/:provider/callback` | Public callback |
| POST | `/integrations/calendar/sync` | Calendar sync |
| POST | `/integrations/email/sync` | Email sync |

Default OAuth redirect URLs (via gateway):

```
http://localhost:8080/api/v1/crm/v1/integrations/oauth/google/callback
http://localhost:8080/api/v1/crm/v1/integrations/oauth/microsoft/callback
```

### 5.10 Admin (staff)

| Method | Path | Notes |
|---|---|---|
| GET | `/admin/audit-logs` | Staff |
| GET | `/admin/monitoring/summary` | Staff |
| GET | `/admin/monitoring/activity` | Staff |
| POST | `/admin/bridge/sync` | Staff |

---

## 6. Pagination & list conventions

Query params: `limit`, `offset`, `q`, `owner`, `stage`, `status`

Response:

```json
{
  "data": [ /* T[] */ ],
  "meta": { "total": 120, "limit": 50, "offset": 0 }
}
```

---

## 7. Realtime (SSE)

`GET /activities/stream` — `text/event-stream`, server polls every ~8s.

**Next.js note:** browser `EventSource` cannot send `Authorization` headers.
Use one of:
- BFF route that proxies SSE with server-side token
- Query param token (only if your gateway policy allows it — CRM default does not)

Pattern matches fleet SSE guidance in
[services/operations/fleet/docs/FRONTEND_INTEGRATION.md](../../operations/fleet/docs/FRONTEND_INTEGRATION.md).

---

## 8. Error Conventions

| Status | Meaning | Frontend action |
|---|---|---|
| 400 | Validation | Inline field error |
| 401 | Missing / invalid token | Refresh → re-login |
| 403 | Permission denied | Hide control |
| 404 | Not found | Soft state |
| 409 | Conflict | Re-fetch |
| 500 | Server error | Toast + retry |
| 503 | Dependency down | Maintenance banner |

---

## 9. TypeScript client

Use [crm-api.ts](./crm-api.ts) as the starting point:

```ts
import { crmApi } from "./crm-api";

const boot = await crmApi.bootstrap(accessToken);
const accounts = await crmApi.accounts.list(accessToken, { limit: 50, offset: 0 });
```

Extend the client for marketing/journey routes as your Next.js app grows.

---

## 10. Quickstart Checklist

- [ ] Set `NEXT_PUBLIC_CRM_API_URL` and `NEXT_PUBLIC_AUTH_API_URL`.
- [ ] Implement OAuth login + silent refresh.
- [ ] Confirm JWT `aud` includes `iag.gateway` and `iag.crm`.
- [ ] Confirm `platform.access_crm` (or superadmin).
- [ ] On app load: `GET /bootstrap` — one round-trip for layout + RBAC.
- [ ] Cache `session.pages` / `session.modals` for nav gating.
- [ ] Use `/permissions/check` for fine-grained button visibility.
- [ ] Wire SSE via BFF if using activity stream.
- [ ] Register OAuth redirect URLs in Google/Microsoft consoles.

---

## See Also

- [README.md](../README.md)
- [crm-api.ts](./crm-api.ts)
- [PRODUCTION_CHECKLIST.md](./PRODUCTION_CHECKLIST.md)
- [docs/RBAC.md](../../../../docs/RBAC.md)
- Auth: [shared/services/authentication](../../../../shared/services/authentication)
- Sibling guide: [contract-management FRONTEND_INTEGRATION.md](../../contract-management/docs/FRONTEND_INTEGRATION.md)
