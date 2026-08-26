#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APPS_DIR="$SCRIPT_DIR"

echo ""
echo "  Select app to deploy:"
echo ""

app_dirs=()
while IFS= read -r dir; do
  app_dirs+=("$dir")
done < <(find "$APPS_DIR" -maxdepth 1 -mindepth 1 -type d -not -name '.*' -not -name 'shared' | sort)

for i in "${!app_dirs[@]}"; do
  app_name=$(basename "${app_dirs[$i]}")
  printf "    %d) %s\n" "$((i + 1))" "$app_name"
done

echo ""
read -rp "  Enter number (1-${#app_dirs[@]}): " choice

if ! [[ "$choice" =~ ^[0-9]+$ ]] || (( choice < 1 || choice > ${#app_dirs[@]} )); then
  echo "Invalid choice: $choice" >&2
  exit 1
fi

app_dir="${app_dirs[$((choice - 1))]}"
app_name=$(basename "$app_dir")

echo ""
echo "  Deploying: $app_name"
echo ""

# Source .env
if [[ -f "$app_dir/.env" ]]; then
  set -a
  source "$app_dir/.env"
  set +a
fi

# App config
case "$app_name" in
  goatcounter)
    required_vars=(DOMAIN EMAIL IMAGE PASSWORD)
    k8s_dir="goatcounter"
    apply_order=(pvc secret services network-policy deployment)
    has_clusterissuer=1
    ;;
  mazanoke)
    required_vars=(DOMAIN EMAIL IMAGE)
    k8s_dir="mazanoke"
    apply_order=(secret services deployment ingress)
    has_clusterissuer=1
    ;;
  mbakmegumi)
    required_vars=(DOMAIN EMAIL IMAGE)
    k8s_dir="mbakmegumi"
    apply_order=(secret services deployment ingress)
    has_clusterissuer=1
    ;;
  ryuko-matoi-go)
    required_vars=(IMAGE REMOVE_BG_API_KEY AI_API_KEY AI_PROVIDER AI_MODEL WHATSAPP_SESSION_PATH WHATSAPP_DATABASE_DIALECT WHATSAPP_EVENT_BUFFER_SIZE)
    k8s_dir="ryuko-matoi-go"
    apply_order=(argocd-application)
    has_clusterissuer=0
    ;;
  ai-assistant)
    required_vars=(IMAGE AI_PROVIDER AI_MODEL TELEGRAM_BOT_TOKEN TELEGRAM_USER_ID)
    : "${JOB_ALERT_PIPELINE_ENABLED:=true}"
    : "${GLINTS_ENABLED:=true}"
    : "${JOB_ALERT_DB_PATH:=/root/.picoclaw/jobs.db}"
    : "${JOB_ALERT_MAX_QUERIES:=5}"
    : "${JOB_ALERT_AI_BATCH_SIZE:=5}"
    : "${JOB_ALERT_MIN_MATCH_SCORE:=1}"
    export JOB_ALERT_PIPELINE_ENABLED GLINTS_ENABLED JOB_ALERT_DB_PATH
    export JOB_ALERT_MAX_QUERIES JOB_ALERT_AI_BATCH_SIZE JOB_ALERT_MIN_MATCH_SCORE
    case "${AI_PROVIDER:-}" in
      sumopod) required_vars+=(SUMOPOD_API_KEY) ;;
      google) required_vars+=(GOOGLE_API_KEY) ;;
      openai) required_vars+=(OPENAI_API_KEY) ;;
      *)
        echo "AI_PROVIDER must be sumopod, google, or openai." >&2
        exit 1
        ;;
    esac
    if [[ -n "${MAIL_TO:-}" ]]; then
      required_vars+=(MAIL_MAILER MAIL_USERNAME MAIL_PASSWORD MAIL_HOST MAIL_PORT MAIL_ENCRYPTION MAIL_FROM MAIL_TO)
      if [[ "${MAIL_MAILER,,}" != "smtp" ]]; then
        echo "MAIL_MAILER must be smtp." >&2
        exit 1
      fi
      if [[ "${MAIL_ENCRYPTION,,}" != "ssl" ]]; then
        echo "MAIL_ENCRYPTION must be ssl." >&2
        exit 1
      fi
    fi
    k8s_dir="ai-assistant"
    apply_order=(pvc secret services network-policy deployment)
    has_clusterissuer=0
    ;;
  argocd)
    required_vars=(DOMAIN EMAIL)
    k8s_dir="argocd"
    apply_order=(argocd-cm secret services ingress)
    has_clusterissuer=1
    ;;
  ekel-backend)
    required_vars=(DOMAIN IMAGE WAKATIME_API_URL WAKATIME_API_KEY TURSO_AUTH_TOKEN TURSO_DATABASE_URL ADMIN_PASSWORD ADMIN_EMAIL JWT_SECRET IHSG_API_URL SECRET_KEY_ADMIN SECRET_KEY_CUSTOMER)
    k8s_dir="ekel-backend"
    apply_order=(secret services deployment ingress)
    has_clusterissuer=1
    ;;
  beszel)
    required_vars=(DOMAIN EMAIL IMAGE KEY TOKEN)
    k8s_dir="beszel"
    apply_order=(argocd-application secret services pvc deployment daemonset ingress)
    has_clusterissuer=1
    ;;
  *)
    echo "Unknown app: $app_name" >&2
    exit 1
    ;;
esac

# Validate required vars
for var in "${required_vars[@]}"; do
  if [[ -z "${!var:-}" ]]; then
    echo "Missing required env var: ${var}. Set it in the shell or .env." >&2
    exit 1
  fi
done

# render_apply
render_apply() {
  if envsubst < "$1" | grep -q '\${'; then
    echo "Rendered ${1} still contains unsubstituted variables. Check your env values." >&2
    exit 1
  fi
  envsubst < "$1" | kubectl apply -f -
}

# Apply
k8s_path="$SCRIPT_DIR"

if [[ "$has_clusterissuer" == 1 ]]; then
  if kubectl api-resources --api-group=cert-manager.io 2>/dev/null | grep -q issuers; then
    cd "$k8s_path/shared"
    render_apply clusterissuer.yaml

    if kubectl api-resources --api-group=traefik.io 2>/dev/null | grep -q middlewares; then
      for mw in middleware/*.yaml; do
        [[ -f "$mw" ]] && kubectl apply -f "$mw"
      done
    else
      echo "  Traefik CRDs not found - skipping middlewares. Install Traefik first." >&2
    fi
  else
    echo "  cert-manager CRDs not found - skipping ClusterIssuer. Install cert-manager first." >&2
  fi
fi

if [[ "$app_name" == "argocd" ]]; then
  argocd_version="${ARGOCD_VERSION:-v3.4.2}"

  kubectl create namespace argocd --dry-run=client -o yaml | kubectl apply -f -
  kubectl apply -n argocd --server-side --force-conflicts \
    -f "https://raw.githubusercontent.com/argoproj/argo-cd/${argocd_version}/manifests/install.yaml"

  echo "  Waiting for ArgoCD pods to be ready..."
  kubectl -n argocd wait --for=condition=Ready pod --all --timeout=3m 2>/dev/null || true

  kubectl -n argocd patch configmap argocd-cmd-params-cm \
    --type merge \
    -p '{"data":{"server.insecure":"true"}}'
  kubectl -n argocd rollout restart deployment/argocd-server

  echo "  Waiting for ArgoCD server after restart..."
  kubectl -n argocd wait --for=condition=Ready pod -l app.kubernetes.io/name=argocd-server --timeout=2m
fi

cd "$k8s_path/$k8s_dir"

for resource in "${apply_order[@]}"; do
  file="${resource}.yaml"
  if [[ -f "$file" ]]; then
    if [[ "$resource" == "pvc" || "$resource" == "services" || "$resource" == "network-policy" || "$resource" == "argocd-application" || "$resource" == "daemonset" ]]; then
      kubectl apply -f "$file"
    else
      render_apply "$file"
    fi
  fi
done

echo ""
echo "  Done: $app_name deployed."
