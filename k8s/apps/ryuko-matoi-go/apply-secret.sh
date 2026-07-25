#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_dir=$(dirname "$script_dir")
namespace=${KUBERNETES_NAMESPACE:-default}

if [ ! -f "$project_dir/.env" ]; then
  echo "Missing $project_dir/.env. Copy .env.example first." >&2
  exit 1
fi

set -a
. "$project_dir/.env"
set +a

: "${APP_NAME:=ryuko-matoi}"
: "${APP_ENV:=production}"
: "${TZ:=Asia/Jakarta}"
: "${LOG_LEVEL:=info}"
: "${LOG_DIR:=./logs}"
: "${WHATSAPP_SESSION_PATH:=./data/session}"
: "${WHATSAPP_DEVICE_NAME:=ryuko-matoi-bot}"
: "${WHATSAPP_DATABASE_DIALECT:=sqlite}"
: "${WHATSAPP_EVENT_BUFFER_SIZE:=64}"
: "${OCR_PROVIDER:=gosseract}"
: "${OCR_LANGUAGE:=eng}"
: "${OCR_BINARY:=tesseract}"
: "${AI_PROVIDER:=google-genai}"
: "${AI_API_KEY:=}"
: "${AI_MODEL:=gemini-2.5-flash}"
: "${REMOVE_BG_API_KEY:=}"
: "${REMOVE_BG_API_URL:=}"
: "${ANIME_QUOTE_API_URL:=}"
: "${JOKES_API_URL:=}"
: "${DISTRO_INFO_API_URL:=}"
: "${DOA_API_URL:=}"
: "${QURAN_API_URL:=}"
: "${IMAGE_API_URL:=}"
: "${ASMAUL_HUSNA_API_URL:=}"
: "${BRAT_FONT_PATH:=}"

case "$WHATSAPP_EVENT_BUFFER_SIZE" in
  *[!0-9]* | '')
    echo "WHATSAPP_EVENT_BUFFER_SIZE must be a non-negative integer." >&2
    exit 1
    ;;
esac

kubectl create secret generic ryuko-matoi-env \
  --namespace "$namespace" \
  --dry-run=client \
  --output yaml \
  --from-literal=APP_NAME="$APP_NAME" \
  --from-literal=APP_ENV="$APP_ENV" \
  --from-literal=TZ="$TZ" \
  --from-literal=LOG_LEVEL="$LOG_LEVEL" \
  --from-literal=LOG_DIR="$LOG_DIR" \
  --from-literal=WHATSAPP_SESSION_PATH="$WHATSAPP_SESSION_PATH" \
  --from-literal=WHATSAPP_DEVICE_NAME="$WHATSAPP_DEVICE_NAME" \
  --from-literal=WHATSAPP_DATABASE_DIALECT="$WHATSAPP_DATABASE_DIALECT" \
  --from-literal=WHATSAPP_EVENT_BUFFER_SIZE="$WHATSAPP_EVENT_BUFFER_SIZE" \
  --from-literal=OCR_PROVIDER="$OCR_PROVIDER" \
  --from-literal=OCR_LANGUAGE="$OCR_LANGUAGE" \
  --from-literal=OCR_BINARY="$OCR_BINARY" \
  --from-literal=AI_PROVIDER="$AI_PROVIDER" \
  --from-literal=AI_API_KEY="$AI_API_KEY" \
  --from-literal=AI_MODEL="$AI_MODEL" \
  --from-literal=REMOVE_BG_API_KEY="$REMOVE_BG_API_KEY" \
  --from-literal=REMOVE_BG_API_URL="$REMOVE_BG_API_URL" \
  --from-literal=ANIME_QUOTE_API_URL="$ANIME_QUOTE_API_URL" \
  --from-literal=JOKES_API_URL="$JOKES_API_URL" \
  --from-literal=DISTRO_INFO_API_URL="$DISTRO_INFO_API_URL" \
  --from-literal=DOA_API_URL="$DOA_API_URL" \
  --from-literal=QURAN_API_URL="$QURAN_API_URL" \
  --from-literal=IMAGE_API_URL="$IMAGE_API_URL" \
  --from-literal=ASMAUL_HUSNA_API_URL="$ASMAUL_HUSNA_API_URL" \
  --from-literal=BRAT_FONT_PATH="$BRAT_FONT_PATH" \
  | kubectl apply -f -

echo "Applied Secret/ryuko-matoi-env in namespace $namespace"
