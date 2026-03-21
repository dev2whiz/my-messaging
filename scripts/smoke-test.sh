#!/usr/bin/env bash
set -euo pipefail

MODE="${1:-docker}" # docker | local

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="${TMPDIR:-/tmp}/my-messaging-smoke"
mkdir -p "$TMP_DIR"

WEB_URL="${WEB_URL:-http://localhost:5173}"
API_URL="${API_URL:-http://localhost:8080}"
PROXY_API_URL="${PROXY_API_URL:-$WEB_URL/api}"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "[smoke] missing required command: $1" >&2
    exit 1
  fi
}

check_http() {
  local name="$1"
  local method="$2"
  local url="$3"
  local expected_code="$4"
  local out_file="$5"
  local data="${6:-}"
  local auth="${7:-}"

  local code
  if [[ -n "$data" && -n "$auth" ]]; then
    code="$(curl -sS -o "$out_file" -w '%{http_code}' -X "$method" "$url" -H 'Content-Type: application/json' -H "Authorization: Bearer $auth" -d "$data")"
  elif [[ -n "$data" ]]; then
    code="$(curl -sS -o "$out_file" -w '%{http_code}' -X "$method" "$url" -H 'Content-Type: application/json' -d "$data")"
  elif [[ -n "$auth" ]]; then
    code="$(curl -sS -o "$out_file" -w '%{http_code}' -X "$method" "$url" -H "Authorization: Bearer $auth")"
  else
    code="$(curl -sS -o "$out_file" -w '%{http_code}' -X "$method" "$url")"
  fi

  if [[ "$code" != "$expected_code" ]]; then
    echo "[smoke] FAIL: $name expected $expected_code got $code" >&2
    if [[ -s "$out_file" ]]; then
      echo "[smoke] response:" >&2
      cat "$out_file" >&2
    fi
    exit 1
  fi
  echo "[smoke] PASS: $name ($code)"
}

cleanup_user_by_token() {
  local token="$1"
  if [[ -z "$token" || "$token" == "null" ]]; then
    return
  fi

  local cleanup_code
  cleanup_code="$(curl -sS -o /dev/null -w '%{http_code}' -X DELETE "$API_URL/auth/me" -H "Authorization: Bearer $token" || true)"
  if [[ "$cleanup_code" == "204" || "$cleanup_code" == "404" ]]; then
    echo "[smoke] cleanup via unregister API returned $cleanup_code"
  else
    echo "[smoke] WARN: cleanup via unregister API returned $cleanup_code" >&2
  fi
}

need_cmd curl
need_cmd jq

if [[ "$MODE" != "docker" && "$MODE" != "local" ]]; then
  echo "Usage: scripts/smoke-test.sh [docker|local]" >&2
  exit 1
fi

echo "[smoke] mode=$MODE"
echo "[smoke] WEB_URL=$WEB_URL"
echo "[smoke] API_URL=$API_URL"
echo "[smoke] PROXY_API_URL=$PROXY_API_URL"

if [[ "$MODE" == "docker" ]]; then
  need_cmd docker
  echo "[smoke] checking docker services"
  docker compose ps --status running >/dev/null
fi

cleanup_test_data() {
  local token
  for token in "${TOKEN_B:-}" "${TOKEN_A:-}"; do
    cleanup_user_by_token "$token"
  done
}

cleanup_stale_smoke_users() {
  if [[ ! -f "$TMP_DIR/users_a.json" ]]; then
    return
  fi

  local stale_found=false
  local username email login_code stale_token
  while IFS= read -r username; do
    [[ -z "$username" ]] && continue
    stale_found=true
    email="${username}@test.local"
    login_code="$(curl -sS -o "$TMP_DIR/stale_login.json" -w '%{http_code}' -X POST "$API_URL/auth/login" -H 'Content-Type: application/json' -d "{\"email\":\"$email\",\"password\":\"$PASS\"}" || true)"
    if [[ "$login_code" != "200" ]]; then
      echo "[smoke] WARN: stale smoke user $username could not be logged in for cleanup (HTTP $login_code)" >&2
      continue
    fi

    stale_token="$(jq -r '.token' "$TMP_DIR/stale_login.json")"
    cleanup_user_by_token "$stale_token"
  done < <(jq -r --arg current "$USER_A" '.[] | select(.username | startswith("smoke_")) | .username | select(. != $current)' "$TMP_DIR/users_a.json")

  if [[ "$stale_found" == true ]]; then
    echo "[smoke] cleaned stale smoke users from previous runs"
  fi
}

on_exit() {
  local status=$?
  cleanup_test_data
  exit "$status"
}

trap on_exit EXIT

SUFFIX="$(date +%s)"
USER_A="smoke_a_${SUFFIX}"
USER_B="smoke_b_${SUFFIX}"
EMAIL_A="${USER_A}@test.local"
EMAIL_B="${USER_B}@test.local"
PASS="password123"

# 1. UI reachability
check_http "UI reachability" "GET" "$WEB_URL" "200" "$TMP_DIR/ui.json"

# 2. Register User A (direct)
check_http "register user A (direct)" "POST" "$API_URL/auth/register" "201" "$TMP_DIR/register_a.json" "{\"username\":\"$USER_A\",\"email\":\"$EMAIL_A\",\"password\":\"$PASS\"}"
jq -e '.token | strings' "$TMP_DIR/register_a.json" >/dev/null
jq -e '.user.id | strings' "$TMP_DIR/register_a.json" >/dev/null
jq -e '.user.username | strings' "$TMP_DIR/register_a.json" >/dev/null
jq -e '.user.email | strings' "$TMP_DIR/register_a.json" >/dev/null
echo "[smoke] PASS: register payload shape"

# 3. Login User A
check_http "login user A" "POST" "$API_URL/auth/login" "200" "$TMP_DIR/login_a.json" "{\"email\":\"$EMAIL_A\",\"password\":\"$PASS\"}"
TOKEN_A="$(jq -r '.token' "$TMP_DIR/login_a.json")"
if [[ -z "$TOKEN_A" || "$TOKEN_A" == "null" ]]; then
  echo "[smoke] FAIL: token missing after login" >&2
  exit 1
fi

echo "[smoke] PASS: login token issued"

# 4. /auth/me
check_http "auth me" "GET" "$API_URL/auth/me" "200" "$TMP_DIR/me_a.json" "" "$TOKEN_A"
jq -e '.id | strings' "$TMP_DIR/me_a.json" >/dev/null
jq -e '.username | strings' "$TMP_DIR/me_a.json" >/dev/null
jq -e '.email | strings' "$TMP_DIR/me_a.json" >/dev/null
echo "[smoke] PASS: me payload shape"

# 5. /users
check_http "list users" "GET" "$API_URL/users" "200" "$TMP_DIR/users_a.json" "" "$TOKEN_A"
jq -e 'type == "array"' "$TMP_DIR/users_a.json" >/dev/null
echo "[smoke] PASS: users payload shape"

cleanup_stale_smoke_users

# 6. Register User B via web proxy
check_http "register user B (proxied)" "POST" "$PROXY_API_URL/auth/register" "201" "$TMP_DIR/register_b_proxy.json" "{\"username\":\"$USER_B\",\"email\":\"$EMAIL_B\",\"password\":\"$PASS\"}"
RECIP_B="$(jq -r '.user.id' "$TMP_DIR/register_b_proxy.json")"
TOKEN_B="$(jq -r '.token' "$TMP_DIR/register_b_proxy.json")"
if [[ -z "$RECIP_B" || "$RECIP_B" == "null" ]]; then
  echo "[smoke] FAIL: recipient id missing" >&2
  exit 1
fi

echo "[smoke] PASS: proxy registration payload shape"

# 7. Send message A -> B (recipient_id is snake_case)
check_http "send message A->B" "POST" "$API_URL/messages" "201" "$TMP_DIR/send_a_b.json" "{\"recipient_id\":\"$RECIP_B\",\"body\":\"hello from smoke\"}" "$TOKEN_A"
jq -e '.id | strings' "$TMP_DIR/send_a_b.json" >/dev/null
jq -e '.conversation_id | strings' "$TMP_DIR/send_a_b.json" >/dev/null
jq -e '.sender_id | strings' "$TMP_DIR/send_a_b.json" >/dev/null
jq -e '.sender_username | strings' "$TMP_DIR/send_a_b.json" >/dev/null
jq -e '.body == "hello from smoke"' "$TMP_DIR/send_a_b.json" >/dev/null
jq -e '.sent_at | strings' "$TMP_DIR/send_a_b.json" >/dev/null
echo "[smoke] PASS: send payload shape"

# 8. Resolve direct conversation + history check
CONV_ID="$(jq -r '.conversation_id' "$TMP_DIR/send_a_b.json")"
check_http "resolve direct conversation" "GET" "$API_URL/conversations/direct/$RECIP_B" "200" "$TMP_DIR/direct_conv.json" "" "$TOKEN_A"
jq -e '.id == "'"$CONV_ID"'"' "$TMP_DIR/direct_conv.json" >/dev/null

check_http "list conversation messages" "GET" "$API_URL/conversations/$CONV_ID/messages" "200" "$TMP_DIR/history_a_b.json" "" "$TOKEN_A"
jq -e 'type == "array" and length >= 1' "$TMP_DIR/history_a_b.json" >/dev/null
jq -e '.[0] | has("conversation_id") and has("sender_id") and has("sender_username") and has("sent_at")' "$TMP_DIR/history_a_b.json" >/dev/null

echo "[smoke] PASS: conversation + history"

echo ""
echo "[smoke] SUCCESS: all sanity checks passed"
echo "[smoke] artifacts: $TMP_DIR"
