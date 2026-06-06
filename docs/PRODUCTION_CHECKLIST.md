# CRM — production checklist

Use this before enabling CRM in staging/production.

## Required

| Item | Env / setting | Verify |
|------|----------------|--------|
| Database | `DATABASE_URL` migrated through `0005_journey_integrations` | `GET /ready` returns `database: true` |
| Auth | `JWT_ISSUER`, `JWKS_URL`, `AUDIENCE=iag.crm` | Mutating API returns 401 without Bearer |
| Service account | `SERVICE_CLIENT_SECRET` (≥16 chars) | Startup log: permissions registered |
| Strict RBAC | `ENVIRONMENT=production` | Tokens without `permissions` are denied |
| Kafka publish | `EVENT_BUS_ENABLED=true`, `KAFKA_BROKERS` | Deal won emits `crm.deal.won` via outbox |
| Kafka consumer | `CONSUMER_ENABLED=true` | Contract/DMS/users events update CRM state |
| DLQ | `CONSUMER_DLQ_TOPIC=iag.dlq.crm` | Failed events land in DLQ topic |
| Finance AR | `FINANCE_API_URL` + service secret | Won deal / signed quote sets `finance_ar_ref` |
| DMS bridge | `DMS_API_URL` | `POST /bridge/sync` upserts outlets from DMS |

## Recommended

| Item | Notes |
|------|--------|
| `PUBLIC_API_URL` | Gateway origin for upstream resolution |
| `USERS_API_URL` | Billing identity resolution for AR |
| `CONTRACTS_API_URL` | Signed quote → contract creation |
| `ALLOWED_ORIGINS` | Explicit CORS list (not `*`) |

## Kubernetes

Manifests: [`deploy/kubernetes/crm/`](../../../../deploy/kubernetes/crm/)

1. Copy `secret.example.yaml` → sealed secret / external secrets operator.
2. Apply configmap + deployment + service.
3. Point gateway `UPSTREAM_CRM` at `iag-crm:4101`.

## Smoke test (post-deploy)

```bash
curl -s https://api.example.com/api/v1/crm/ready

curl -s -H "Authorization: Bearer $TOKEN" \
  https://api.example.com/api/v1/crm/v1/overview

# Bridge sync (requires DMS upstream)
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  https://api.example.com/api/v1/crm/v1/bridge/sync
```

## Journey runner & integrations

| Item | Env / route | Verify |
|------|-------------|--------|
| Journey runner | `JOURNEY_RUNNER_INTERVAL=30s` | New contact auto-enrolls JRN-001; enrollments advance |
| Journey API | `POST /v1/journeys/:id/enroll` | Enrollment row + `crm.journey.enrolled` event |
| Google OAuth | `GOOGLE_OAUTH_*` + redirect URL | `GET /v1/integrations/oauth/google/start` returns `authorize_url` |
| OAuth state signing | `SERVICE_CLIENT_SECRET` or `INTEGRATION_TOKEN_SECRET` (≥16 chars) | Callback rejects unsigned/expired state in production |
| Integration tokens | `INTEGRATION_TOKEN_SECRET` (defaults to service secret) | OAuth tokens stored AES-GCM sealed (`enc1:` prefix) in Postgres |
| Calendar sync | Connected Google/Microsoft | `POST /v1/integrations/calendar/sync` imports activities |
| Email sync | Connected provider | `POST /v1/integrations/email/sync` touches matching contacts |
| Demand-gen metrics | `GET /v1/marketing/demand-gen` | Channel CPL/MQL counts from `crm_mqls` + budget plans |
| Marketing hub | `GET /v1/marketing/hub` | `budget_burn_pct` from budget plans vs campaign/event spend |
| Overview charts | `GET /v1/overview?range=week` | `metrics.series` / `x_labels` from live deal pipeline buckets |

## Commercial loop

- **Deal won** → finance AR (`CRM-DEAL-{id}`) + `crm.deal.won`
- **Quote signed** → finance AR + contract-management + `crm.quote.signed`
- **DMS outlet created** (consumer) → pending import row
- **Contract created** (consumer) → `quote.contract_ref` when `crmQuoteId` present
