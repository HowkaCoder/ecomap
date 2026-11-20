# EcoMap

Telegram bot that helps keep a city clean. Residents register their trash bins and get a QR code for each one, mark bins as full when they need to be emptied, and report illegal dumps with a photo and location.

![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-pgx-4169E1?logo=postgresql&logoColor=white)
![Telegram](https://img.shields.io/badge/Telegram-Bot_API-26A5E4?logo=telegram&logoColor=white)
[![ci](https://github.com/HowkaCoder/ecomap/actions/workflows/ci.yml/badge.svg)](https://github.com/HowkaCoder/ecomap/actions/workflows/ci.yml)

## How it works

**Registration:** name → one representative photo → address → location. The bot creates three material-specific bins (plastic, paper/cardboard, glass) and sends a separate deep-link QR for each one.

**My bins:** list of the user's bins with buttons «Full» / «Empty». Only the owner can change the status of a bin.

**Pickup workflow:** a collector opens a material batch with `/batch`, scans a bin QR, records its weight, and closes the batch with `/finishbatch`. A recycler confirms accepted weight with `/accept <batch-code> <kg>`. Residents can see verified accepted weight in `/impact`.

**Collection calendar:** set `PICKUP_SCHEDULE=mon:plastic,tue:paper,fri:glass`. Residents can view it with `/calendar` and enable or disable reminders with `/reminders`. Timezone and reminder hour are configurable.

**Missed pickup and dump reports:** residents can submit a missed-pickup issue or a dump report, then track its status with `/myreports`. Admins can list and close issues with `/issues`, `/dumps`, `/resolve_issue <id>`, and `/resolve_dump <id>`.

**Sorting guide:** `/sort` explains the pilot's basic material rules.

## Architecture

Clean architecture with ports and adapters. Business logic does not know about Telegram or PostgreSQL:

```
cmd/                      entrypoint, wiring, graceful shutdown
internal/
  domain/                 entities and repository interfaces (user, bin, dump)
  usecase/                registration, bin status, dump reports
  dto/                    input data for use cases
  adapter/
    postgres/             repositories on pgx
    photo/                downloads photos from Telegram
    qr/                   generates QR codes
  handler/telegram/       conversation state machine
migrations/               database schema
```

Use cases depend only on interfaces, so they are tested with in-memory fakes and no database.

## Getting started

```bash
cp .env.example .env    # put your BOT_TOKEN from @BotFather
docker compose up --build
```

The schema from `migrations/` is applied automatically on the first start of the database container.

Without Docker:

```bash
psql "$DATABASE_URL" -f migrations/001_init.sql
go run ./cmd
```

| Variable | Description |
|---|---|
| `BOT_TOKEN` | Telegram bot token |
| `DATABASE_URL` | PostgreSQL connection string |
| `STORAGE_DIR` | Where photos and QR codes are saved, `storage` by default |
| `ADMIN_TELEGRAM_IDS` | Comma-separated Telegram IDs with full pilot access |
| `COLLECTOR_TELEGRAM_IDS` | Comma-separated IDs allowed to record collections |
| `RECYCLER_TELEGRAM_IDS` | Comma-separated IDs allowed to confirm accepted batch weight |
| `PICKUP_SCHEDULE` | Weekly schedule, e.g. `mon:plastic,tue:paper,fri:glass` |
| `TIMEZONE` | Schedule timezone, defaults to `Asia/Tashkent` |
| `REMINDER_HOUR` | Local reminder hour, defaults to `19` |

Railway exposes a public health endpoint at `/healthz`. When `BOT_TOKEN` is not set, the service stays healthy but waits for Telegram configuration; it does not start polling.

## Tests

```bash
go test ./...
```
