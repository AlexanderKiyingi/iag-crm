#!/usr/bin/env sh
# Smoke test CRM via gateway (requires stack running on localhost:8080).
set -eu

BASE="${CRM_GATEWAY_URL:-http://localhost:8080/api/v1/crm}"
TOKEN="${CRM_SMOKE_TOKEN:-}"

check() {
  method=$1
  path=$2
  expect=$3
  code=$(curl -s -o /dev/null -w "%{http_code}" -X "$method" \
    ${TOKEN:+ -H "Authorization: Bearer $TOKEN"} \
    "$BASE$path")
  if [ "$code" != "$expect" ]; then
    echo "FAIL $method $path expected $expect got $code"
    exit 1
  fi
  echo "OK   $method $path -> $code"
}

check GET /health 200
check GET /ready 200

if [ -n "$TOKEN" ]; then
  check GET /v1/bootstrap 200
  check GET /v1/accounts 200
  check GET /v1/audit 200
else
  echo "skip authenticated routes (set CRM_SMOKE_TOKEN to test /v1/*)"
fi

echo "CRM smoke test passed"
