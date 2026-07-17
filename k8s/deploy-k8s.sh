#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APPS_DIR="$SCRIPT_DIR/apps"

echo ""
echo "  Select app to deploy:"
echo ""

app_dirs=()
while IFS= read -r dir; do
  app_dirs+=("$dir")
done < <(find "$APPS_DIR" -maxdepth 1 -mindepth 1 -type d | sort)

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
    apply_order=(pvc secret services deployment ingress)
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
  omniroute)
    required_vars=(DOMAIN IMAGE OMNIROUTE_WS_BRIDGE_SECRET)
    k8s_dir="omniroute"
    apply_order=(pvc secret services deployment ingress)
    has_clusterissuer=1
    ;;
  ryuko-matoi-go)
    required_vars=(IMAGE)
    k8s_dir="ryuko-matoi-go"
    apply_order=(pvc secret services deployment)
    has_clusterissuer=0
    ;;
  umami)
    required_vars=(POSTGRES_USER POSTGRES_PASSWORD POSTGRES_DB APP_SECRET DOMAIN EMAIL TRACKER_SCRIPT_NAME)
    k8s_dir="umami"
    apply_order=(secret services postgres networkpolicy deployment ingress)
    has_clusterissuer=1
    ;;
  ekel-backend)
    required_vars=(DOMAIN IMAGE WAKATIME_API_URL WAKATIME_API_KEY TURSO_AUTH_TOKEN TURSO_DATABASE_URL ADMIN_PASSWORD ADMIN_EMAIL JWT_SECRET)
    k8s_dir="ekel-backend"
    apply_order=(secret services deployment ingress)
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
k8s_path="$SCRIPT_DIR/k8s"

if [[ "$has_clusterissuer" == 1 ]]; then
  cd "$k8s_path/shared"
  render_apply clusterissuer.yaml
fi

cd "$k8s_path/$k8s_dir"

for resource in "${apply_order[@]}"; do
  file="${resource}.yaml"
  if [[ -f "$file" ]]; then
    if [[ "$resource" == "pvc" || "$resource" == "services" || "$resource" == "postgres" || "$resource" == "networkpolicy" ]]; then
      kubectl apply -f "$file"
    else
      render_apply "$file"
    fi
  fi
done

echo ""
echo "  Done: $app_name deployed."
