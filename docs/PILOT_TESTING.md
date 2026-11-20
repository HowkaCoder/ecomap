# EcoMap Bin4Home pilot: test guide

This branch adds a pilot workflow for household recycling and municipal collection. It is isolated from the production branch and database.

## Before the first Telegram test

1. Create a separate test bot with `@BotFather`. Do not reuse the production bot token: Telegram permits only one active long-polling consumer per token.
2. Add the token to the Railway service as `BOT_TOKEN`.
3. Start a chat with the test bot and send `/start`, then `/id`.
4. Add your ID to `ADMIN_TELEGRAM_IDS`. Add pilot collectors to `COLLECTOR_TELEGRAM_IDS` and receiving-facility accounts to `RECYCLER_TELEGRAM_IDS`. IDs are comma-separated. Admins also have collector and recycler permissions.
5. Set `PICKUP_SCHEDULE`, for example `mon:plastic,tue:paper,fri:glass`, and keep `TIMEZONE=Asia/Tashkent`.
6. After changing variables, wait until Railway reports the bot service as online. Check `<service-domain>/healthz`; expected response is `{"status":"ok"}`.

`DATABASE_URL` should be wired to the separate Postgres service. The bot applies additive schema migrations during startup. Do not point this test service at production data.

## Resident scenarios

| Scenario | Steps | Expected result |
|---|---|---|
| Register a household | `/start` → Registration → name → photo → address → location | Three QR codes arrive, one each for plastic, paper and glass; `/bins` lists them separately. |
| See the pickup schedule | Open “График вывоза” or send `/calendar` | Configured weekdays and material types appear; empty schedule shows setup guidance. |
| Enable reminders | `/reminders` → “Включить” | Preference is saved. At the configured local hour, a reminder is sent the evening before a scheduled pickup. |
| Report a missed pickup | “Не забрали” → select a bin → describe the issue | A numbered case is created and appears in `/myreports`. |
| Report an illegal dump | “Сообщить о свалке” → photo → location → address → landmark → description → urgency | Case number is returned and the report appears in `/myreports`. |
| Read the sorting guide | “Что куда?” or `/sort` | Guidance for plastic, paper and glass appears. |
| Check verified eco-balance | `/impact` | Shows no value until a recycler records accepted kilograms; then shows accepted weight by material. |

## Operator and collection scenarios

| Scenario | Steps | Expected result |
|---|---|---|
| Collector opens a route | On an allowlisted collector account, `/batch` → choose material | A route code is issued. Repeating `/batch` returns the same open route. |
| Record a collection | Scan the corresponding household QR with the phone camera → open the test bot → enter a positive weight | The route total increases; the resident gets a notification; that bin becomes empty. A QR for another material or a duplicate bin in one route is rejected. |
| Submit a route | `/finishbatch` | Route changes to submitted; its code and collected weight are shown. Empty routes cannot be submitted. |
| Record facility acceptance | On an allowlisted recycler account, `/accept ROUTE_CODE ACCEPTED_KG` | Accepted, partial, or rejected state is recorded; accepted kg cannot exceed collected kg. |
| Resolve resident cases | On an admin account, `/issues` or `/dumps` → close a case | Case leaves the open queue and its owner sees the resolved status in `/myreports`. |
| Check role boundaries | Repeat operator commands from a resident account | Collection and admin actions are denied. |

## Commands

- Resident: `/start`, `/id`, `/bins`, `/calendar`, `/missed`, `/sort`, `/impact`, `/myreports`, `/reminders`, `/reminders on`, `/reminders off`.
- Collector: `/batch`, scan QR, enter weight, `/finishbatch`.
- Recycler: `/accept ECO-YYYYMMDD-00001 12.5`.
- Admin: `/issues`, `/dumps`, `/resolve_issue ID`, `/resolve_dump ID`.

## Pilot limitations to check with the project owner

- This is a workflow pilot, not an audited municipal dispatch system. It does not yet connect to a real route optimizer, weighbridge, or recycler API.
- The schedule is shared at service level, not assigned to individual neighborhoods.
- The first registration flow creates three household containers using one address, location and photo. Confirm that this matches the actual operating model before broader rollout.
- Accepted kilograms are allocated proportionally across route pickups for the resident eco-balance. This is an estimate when the facility reports a different total weight; the per-bin weights remain available in collection events.
- Telegram bot messages and Telegram IDs are required for the pilot. Configure only test accounts and a test token in Railway.
- There is no Mini App or translation layer in this branch. These are separate product increments.

## Quick operational checks

- Open `https://<service-domain>/` to confirm the test service page loads.
- Open `https://<service-domain>/healthz`; require HTTP 200 and status `ok` after configuring the bot token.
- Railway deployment must show a successful build and a running replica.
- Review logs for database migration errors, Telegram polling errors, and rejected test cases.
- Verify that no production token or production database URL is present in the test project.
