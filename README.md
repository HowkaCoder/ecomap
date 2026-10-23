# EcoMap

Telegram bot that helps keep a city clean. Residents register their trash bins and get a QR code for each one, mark bins as full when they need to be emptied, and report illegal dumps with a photo and location.

![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-pgx-4169E1?logo=postgresql&logoColor=white)
![Telegram](https://img.shields.io/badge/Telegram-Bot_API-26A5E4?logo=telegram&logoColor=white)
[![ci](https://github.com/HowkaCoder/ecomap/actions/workflows/ci.yml/badge.svg)](https://github.com/HowkaCoder/ecomap/actions/workflows/ci.yml)

## How it works

**Registration:** name → photo of the bin → location → the bot sends back a QR code that is printed and stuck on the bin.

**My bins:** list of the user's bins with buttons «Full» / «Empty». Only the owner can change the status of a bin.

**Dump report:** photo → location → address → landmark → description → severity. Reports are stored with coordinates for the city service.

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

## Tests

```bash
go test ./...
```
