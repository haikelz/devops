#!/bin/bash
set -euo pipefail

export TZ=Asia/Jakarta
BASE="/home/ubuntu/ryuko-matoi-go"
JOURNAL_DIR="$BASE/journal"

DATE_STR=13-04-2026
JOURNAL_FILE="$JOURNAL_DIR/journal-${DATE_STR}.log"
INPUT_FILE="$JOURNAL_DIR/nightly-insight-input-${DATE_STR}.md"
INSIGHT_FILE="$JOURNAL_DIR/insight-${DATE_STR}.md"

if [[ ! -f "$JOURNAL_FILE" ]]; then
  echo "journal file not found: $JOURNAL_FILE" >&2
  exit 1
fi

{
  cat <<'EOF'
Anda adalah SRE + backend analyst untuk project Ryuko Matoi (Go WhatsApp bot).

Konteks sistem:
- Bahasa: Go
- Runtime: Docker
- Komponen utama: WhatsApp runtime (whatsmeow), command handler, integrasi AI/OCR, media downloader
- Fokus analisis: stabilitas, error operasional, regresi perilaku command, ketergantungan API eksternal, dan kualitas
observability.

Tugas:
Analisis LOG HARI INI yang dilampirkan, lalu keluarkan insight operasional dalam bahasa Indonesia, format markdown.

Output wajib:
1. Ringkasan Eksekutif (5-8 bullet)
2. Timeline Kejadian Penting (urut waktu)
3. Insiden/Error Utama
   - severity: High/Medium/Low
   - gejala
   - dugaan akar masalah
   - dampak user
4. Pola Berulang / Anti-pattern
5. Health Check Sistem Hari Ini (score 0-100 + alasan)
6. Rekomendasi Aksi
   - P0 (segera)
   - P1 (minggu ini)
   - P2 (improvement)
7. Rekomendasi Logging/Metric/Alert tambahan yang spesifik untuk codebase ini.
8. Lampiran:
   - top 10 error message paling sering
   - top command yang paling sering dipakai (jika terdeteksi)
   - gap data yang menghambat analisis

Aturan:
- Jangan halusinasi. Jika data tidak ada, tulis "tidak terdeteksi di log".
- Sertakan bukti singkat (potongan baris log relevan) untuk tiap temuan penting.
- Fokus pada insight yang actionable.
EOF
  printf '\n\n--- BEGIN JOURNAL (%s) ---\n\n' "$DATE_STR"
  cat "$JOURNAL_FILE"
} > "$INPUT_FILE"

PAYLOAD=$(cat "$INPUT_FILE")

if [[ "${NIGHTLY_INSIGHT_SKIP_PICOCAW:-}" != "1" ]]; then
  if [[ "${NIGHTLY_INSIGHT_PICOCAW_IMMEDIATE:-}" == "1" ]]; then
    picoclaw agent -m "$PAYLOAD" | tee "$INSIGHT_FILE"
    if [[ ! -s "$INSIGHT_FILE" ]]; then
      echo "peringatan: $INSIGHT_FILE kosong — picoclaw mungkin tidak menulis jawaban ke stdout; cek keluaran terminal atau opsi picoclaw lain." >&2
    fi
  else
    picoclaw cron add \
      --name nightly-insight \
      --cron "0 22 * * *" \
      --message "$PAYLOAD"
  fi
else
  echo "picoclaw dilewati (NIGHTLY_INSIGHT_SKIP_PICOCAW=1)" >&2
fi

echo "input siap: $INPUT_FILE"
if [[ "${NIGHTLY_INSIGHT_SKIP_PICOCAW:-}" != "1" ]] && [[ "${NIGHTLY_INSIGHT_PICOCAW_IMMEDIATE:-}" == "1" ]]; then
  echo "insight: $INSIGHT_FILE"
fi
