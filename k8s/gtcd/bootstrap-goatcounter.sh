#!/usr/bin/env bash
# Bootstrap the in-cluster GoatCounter for gtcd: create the site + admin user,
# create/reuse the gtcd-dashboard API token, and write the token into .env.
#
# Run AFTER `deploy-k8s.sh` has deployed the stack (the goatcounter pod must be
# running). Token values are never printed; they go straight into .env.
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="$DIR/.env"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Missing $ENV_FILE — copy .env.example to .env and fill it in first." >&2
  exit 1
fi

# .env already contains DOMAIN and EMAIL; load them into this shell.
set -a
source "$ENV_FILE"
set +a

: "${DOMAIN:?Set DOMAIN in .env}"
: "${EMAIL:?Set EMAIL in .env}"

# Admin password for the GoatCounter site; prompt when not provided via env.
if [[ -z "${GC_ADMIN_PASSWORD:-}" ]]; then
  read -rsp "GoatCounter admin password for ${EMAIL}: " GC_ADMIN_PASSWORD
  echo ""
fi
if [[ ${#GC_ADMIN_PASSWORD} -lt 8 ]]; then
  echo "Password must be at least 8 characters." >&2
  exit 1
fi

echo "Creating site + admin user for ${DOMAIN}..."
kubectl exec deploy/goatcounter -- goatcounter db create site \
  -vhost "$DOMAIN" \
  -user.email "$EMAIL" \
  -user.password "$GC_ADMIN_PASSWORD" \
  -createdb 2>&1 | grep -v "already exists" || true

# Resolve the real user id instead of assuming 1.
USER_ID=$(kubectl exec deploy/goatcounter -- goatcounter db query \
  "select user_id from users" -format json 2>/dev/null \
  | tr -d '\n\t\r ' \
  | grep -o '"user_id":[0-9]*' \
  | head -1 \
  | cut -d: -f2 || true)
USER_ID="${USER_ID:-1}"
echo "User id: ${USER_ID}"

# Reuse an existing token so repeated runs do not pile up duplicates.
# `db show apitoken -format json` prints pretty-printed JSON; compact it first.
fetch_token() {
  kubectl exec deploy/goatcounter -- goatcounter db show apitoken \
    -find 1 -format json 2>/dev/null \
    | tr -d '\n\t\r ' \
    | grep -o '"token":"[^"]*"' \
    | head -1 \
    | cut -d'"' -f4 || true
}

TOKEN=$(fetch_token)

if [[ -n "$TOKEN" ]]; then
  echo "Reusing existing API token."
else
  echo "Creating API token..."
  kubectl exec deploy/goatcounter -- goatcounter db create apitoken \
    -name gtcd-dashboard \
    -user "$USER_ID" \
    -perm "count,export,site_read,site_create,site_update"

  # The CLI cannot grant the 'stats' bit; set all permission bits directly.
  kubectl exec deploy/goatcounter -- goatcounter db query \
    "UPDATE api_tokens SET permissions='127' WHERE name='gtcd-dashboard'" \
    -format exec

  TOKEN=$(fetch_token)
fi

if [[ -z "$TOKEN" ]]; then
  echo "Could not obtain an API token; .env was NOT modified." >&2
  exit 1
fi

python3 - "$ENV_FILE" "$TOKEN" <<'PY'
import pathlib, re, sys

env_path, token = pathlib.Path(sys.argv[1]), sys.argv[2]
text = env_path.read_text()
if re.search(r"^GOATCOUNTER_API_KEY=", text, flags=re.M):
    text = re.sub(r"^GOATCOUNTER_API_KEY=.*$", "GOATCOUNTER_API_KEY=" + token, text, flags=re.M)
else:
    text = text.rstrip("\n") + "\nGOATCOUNTER_API_KEY=" + token + "\n"
env_path.write_text(text)
PY

echo "GOATCOUNTER_API_KEY written to .env (value never displayed)."
echo "Now re-run deploy-k8s.sh (choose gtcd) to update the Secret."
