#!/usr/bin/env bash
set -euo pipefail

app_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ -f "$app_dir/.env" ]]; then
  set -a
  source "$app_dir/.env"
  set +a
fi

required_vars=(POSTGRES_USER POSTGRES_PASSWORD POSTGRES_DB APP_SECRET DOMAIN EMAIL TRACKER_SCRIPT_NAME)
for var in "${required_vars[@]}"; do
  if [[ -z "${!var:-}" ]]; then
    echo "Missing required env var: ${var}. Set it in the shell or .env." >&2
    exit 1
  fi
done

cd "$app_dir/../../k8s/umami"

render_apply() {
  if envsubst < "$1" | grep -q '\${'; then
    echo "Rendered ${1} still contains unsubstituted variables. Check your env values." >&2
    exit 1
  fi

  envsubst < "$1" | kubectl apply -f -
}

cd "$app_dir/../../k8s/shared"
render_apply clusterissuer.yaml

cd ../umami

render_apply secret.yaml
kubectl apply -f services.yaml
kubectl apply -f postgres.yaml
kubectl apply -f networkpolicy.yaml
kubectl apply -f deployment.yaml
render_apply ingress.yaml
