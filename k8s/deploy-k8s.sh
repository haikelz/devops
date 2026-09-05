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
  gtcd)
    required_vars=(DOMAIN EMAIL IMAGE GOATCOUNTER_URL GOATCOUNTER_API_KEY SESSION_SECRET)
    : "${REDIS_URL:=}"
    export REDIS_URL
    k8s_dir="gtcd"
    apply_order=(services middleware goatcounter redis deployment ingress)
    has_clusterissuer=1
    secret_name="gtcd-env"
    secret_namespace="default"
    secret_vars=(DOMAIN GOATCOUNTER_URL GOATCOUNTER_API_KEY SESSION_SECRET REDIS_URL)
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
    : "${REMOVE_BG_API_URL:=}"
    : "${JOKES_API_URL:=}"
    : "${ANIME_QUOTE_API_URL:=}"
    : "${DISTRO_INFO_API_URL:=}"
    : "${DOA_API_URL:=}"
    : "${QURAN_API_URL:=}"
    : "${IMAGE_API_URL:=}"
    : "${ASMAUL_HUSNA_API_URL:=}"
    : "${APP_NAME:=ryuko-matoi}"
    : "${APP_ENV:=production}"
    : "${TZ:=Asia/Jakarta}"
    : "${LOG_LEVEL:=info}"
    : "${LOG_DIR:=/app/logs}"
    : "${WHATSAPP_DEVICE_NAME:=RyukoMatoi}"
    : "${OCR_PROVIDER:=}"
    : "${OCR_LANGUAGE:=}"
    : "${OCR_BINARY:=}"
    : "${BRAT_FONT_PATH:=}"
    export REMOVE_BG_API_URL JOKES_API_URL ANIME_QUOTE_API_URL DISTRO_INFO_API_URL DOA_API_URL QURAN_API_URL IMAGE_API_URL ASMAUL_HUSNA_API_URL APP_NAME APP_ENV TZ LOG_LEVEL LOG_DIR WHATSAPP_DEVICE_NAME OCR_PROVIDER OCR_LANGUAGE OCR_BINARY BRAT_FONT_PATH
    k8s_dir="ryuko-matoi-go"
    apply_order=(argocd-application)
    has_clusterissuer=0
    secret_name="ryuko-matoi-env"
    secret_namespace="bots"
    secret_vars=(REMOVE_BG_API_KEY REMOVE_BG_API_URL JOKES_API_URL ANIME_QUOTE_API_URL DISTRO_INFO_API_URL DOA_API_URL QURAN_API_URL IMAGE_API_URL ASMAUL_HUSNA_API_URL AI_PROVIDER AI_API_KEY AI_MODEL APP_NAME APP_ENV TZ LOG_LEVEL LOG_DIR WHATSAPP_SESSION_PATH WHATSAPP_DEVICE_NAME WHATSAPP_DATABASE_DIALECT WHATSAPP_EVENT_BUFFER_SIZE OCR_PROVIDER OCR_LANGUAGE OCR_BINARY BRAT_FONT_PATH)
    ;;
  ai-assistant)
    required_vars=(IMAGE AI_PROVIDER AI_MODEL TELEGRAM_BOT_TOKEN TELEGRAM_USER_ID)
    : "${WHATSAPP_RECIPIENT:=}"
    : "${JOB_ALERT_PIPELINE_ENABLED:=true}"
    : "${GLINTS_ENABLED:=true}"
    : "${LINKEDIN_ENABLED:=false}"
    : "${LINKEDIN_PAGES:=2}"
    : "${LINKEDIN_MAX_DETAILS:=3}"
    : "${LINKEDIN_POSTED_WITHIN_HOURS:=168}"
    : "${LINKEDIN_DISTANCE:=25}"
    : "${LINKEDIN_JOB_TYPES:=}"
    : "${LINKEDIN_COMPANY_IDS:=}"
    : "${JOB_ALERT_DB_PATH:=/root/.picoclaw/jobs.db}"
    : "${JOB_ALERT_MAX_QUERIES:=5}"
    : "${JOB_ALERT_AI_BATCH_SIZE:=5}"
    : "${JOB_ALERT_MIN_MATCH_SCORE:=1}"
    : "${MAIL_MAILER:=}"
    : "${MAIL_USERNAME:=}"
    : "${MAIL_PASSWORD:=}"
    : "${MAIL_HOST:=}"
    : "${MAIL_PORT:=}"
    : "${MAIL_ENCRYPTION:=}"
    : "${MAIL_FROM:=}"
    export WHATSAPP_RECIPIENT JOB_ALERT_PIPELINE_ENABLED GLINTS_ENABLED LINKEDIN_ENABLED
    export LINKEDIN_PAGES LINKEDIN_MAX_DETAILS LINKEDIN_POSTED_WITHIN_HOURS
    export LINKEDIN_DISTANCE LINKEDIN_JOB_TYPES LINKEDIN_COMPANY_IDS JOB_ALERT_DB_PATH
    export JOB_ALERT_MAX_QUERIES JOB_ALERT_AI_BATCH_SIZE JOB_ALERT_MIN_MATCH_SCORE
    export MAIL_MAILER MAIL_USERNAME MAIL_PASSWORD MAIL_HOST MAIL_PORT MAIL_ENCRYPTION MAIL_FROM
    case "${AI_PROVIDER:-}" in
      sumopod) required_vars+=(SUMOPOD_API_KEY) ; : "${SUMOPOD_API_KEY:=}" ; export SUMOPOD_API_KEY ;;
      google) required_vars+=(GOOGLE_API_KEY) ; : "${GOOGLE_API_KEY:=}" ; export GOOGLE_API_KEY ;;
      openai) required_vars+=(OPENAI_API_KEY) ; : "${OPENAI_API_KEY:=}" ; export OPENAI_API_KEY ;;
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
    apply_order=(argocd-application)
    has_clusterissuer=0
    secret_name="ai-assistant-env"
    secret_namespace="bots"
    secret_vars=(IMAGE AI_PROVIDER AI_MODEL TELEGRAM_BOT_TOKEN TELEGRAM_USER_ID WHATSAPP_RECIPIENT JOB_ALERT_PIPELINE_ENABLED GLINTS_ENABLED LINKEDIN_ENABLED LINKEDIN_PAGES LINKEDIN_MAX_DETAILS LINKEDIN_POSTED_WITHIN_HOURS LINKEDIN_DISTANCE LINKEDIN_JOB_TYPES LINKEDIN_COMPANY_IDS JOB_ALERT_DB_PATH JOB_ALERT_MAX_QUERIES JOB_ALERT_AI_BATCH_SIZE JOB_ALERT_MIN_MATCH_SCORE MAIL_MAILER MAIL_USERNAME MAIL_PASSWORD MAIL_HOST MAIL_PORT MAIL_ENCRYPTION MAIL_FROM MAIL_TO SUMOPOD_API_KEY GOOGLE_API_KEY OPENAI_API_KEY)
    ;;
  argocd)
    required_vars=(DOMAIN EMAIL)
    k8s_dir="argocd"
    apply_order=(argocd-cm secret services ingress)
    has_clusterissuer=1
    ;;
  ekel-backend)
    required_vars=(DOMAIN IMAGE WAKATIME_API_URL WAKATIME_API_KEY TURSO_AUTH_TOKEN TURSO_DATABASE_URL ADMIN_PASSWORD ADMIN_EMAIL JWT_SECRET IHSG_API_URL SECRET_KEY_ADMIN SECRET_KEY_CUSTOMER APP_ENV APP_DEBUG PORT)
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

# Create k8s secret
create_k8s_secret() {
  local secret_name="$1"
  local secret_namespace="$2"
  shift 2
  local -a vars=("$@")

  echo "  Creating/updating Secret '$secret_name' in namespace '$secret_namespace'..."
  kubectl create namespace "$secret_namespace" --dry-run=client -o yaml | kubectl apply -f -

  local -a literal_args=()
  for var in "${vars[@]}"; do
    local val="${!var:-}"
    literal_args+=("--from-literal=${var}=${val}")
  done

  kubectl create secret generic "$secret_name" -n "$secret_namespace" \
    "${literal_args[@]}" \
    --dry-run=client -o yaml | kubectl apply -f -
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

if [[ -n "${secret_name:-}" && -n "$secret_vars:-}" && ${#secret_vars[@]} -gt 0 ]]; then
  create_k8s_secret "$secret_name" "${secret_namespace:-default}" "${secret_vars[@]}"
fi

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
