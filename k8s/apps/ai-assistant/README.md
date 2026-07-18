# AI Assistant

PicoClaw Telegram assistant using Sumopod's Responses API with `deepseek-v4-pro` and its own local finance API.

Set `.env` from `.env.example`, then run `./deploy-k8s.sh` and select `ai-assistant`.

Required secrets:

- `SUMOPOD_API_KEY`
- `TELEGRAM_BOT_TOKEN`
- `TELEGRAM_USER_ID` from Telegram's numeric user ID

The assistant only accepts the configured Telegram user. Its state, finance database, and cron jobs persist in the `ai-assistant-data` PVC. This ledger is independent from `ryuko-matoi-go`.

The paycheck reminder runs at 09:00 Asia/Jakarta every 28th. `/downloadrecap` sends an XLSX file that opens directly in Google Sheets.
