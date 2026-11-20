# EcoMap Bin4Home pilot: test guide

This branch adds a pilot workflow for household recycling, collection operations, and a Telegram Mini App. It is isolated from the production branch and database. The Mini App opens from `/start`; Karakalpak is the default language, with Russian available from `🗣 Til / Язык` or the language toggle in the app. Residents can report a missed pickup, report an illegal dump with location, and manage reminders directly in the Mini App.

## Before the first Telegram test

1. The test Railway service already has `BOT_TOKEN` configured. Keep its value private.
2. Start a chat with the configured bot and send `/start`, then `/id`.
3. Confirm your ID is in `ADMIN_TELEGRAM_IDS`. Add pilot collectors to `COLLECTOR_TELEGRAM_IDS` and receiving-facility accounts to `RECYCLER_TELEGRAM_IDS` if other test accounts will be used. IDs are comma-separated. Admins also have collector and recycler permissions.
4. `PICKUP_SCHEDULE` is set to a demo schedule (`mon:plastic,tue:paper,fri:glass`) with `TIMEZONE=Asia/Tashkent`; replace it if you want to test another calendar.
5. Check `<service-domain>/healthz`; expected response is `{"status":"ok"}`. In Telegram, `/start` should show both the Karakalpak keyboard and the Mini App launch button.

`DATABASE_URL` should be wired to the separate Postgres service. The bot applies additive schema migrations during startup. Do not point this test service at production data.

## Resident scenarios

| Scenario | Steps | Expected result |
|---|---|---|
| Register a household | `/start` → `Úydi qosıw` → name → photo → address → location | Three QR codes arrive, one each for plastic, paper and glass; `/bins` and the Mini App list them separately. |
| Open the Mini App | Tap `EcoMap Mini App` under `/start` | A responsive dashboard opens inside Telegram. Karakalpak is selected by default; card transitions and animated charts respect reduced-motion settings. |
| Switch the language | Tap `🗣 Til / Язык` or the Mini App language toggle | Russian and Karakalpak can be selected; the Mini App remembers the choice and the bot stores it for that Telegram account. |
| See the pickup schedule | Open `Alıp ketiw kestesi` or send `/calendar` | Configured weekdays and material types appear; the Mini App formats the schedule for the selected language. |
| Enable reminders | Mini App → `Esletpelerdi qosıw` / `Включить напоминания` | Preference is saved. At the configured local hour, a reminder is sent the evening before a scheduled pickup. The button toggles to disable reminders. |
| Report a missed pickup | Mini App → `Alıp ketiwdi xabar etiw` / `Сообщить о пропуске` → select a bin → describe the issue | A numbered case is created and appears in `/myreports` and the Mini App. An open duplicate for the same bin is rejected. |
| Report an illegal dump | Mini App → `Qoqıs haqqında xabar` / `Сообщить о свалке` → address → description → allow location | A numbered case with coordinates is saved and appears in `/myreports` and the Mini App. Location permission is required. |
| Read the sorting guide | `Qalay sortlaw?` or `/sort` | Guidance for plastic, paper and glass appears. |
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

## Admin dashboard

1. From an account listed in `ADMIN_TELEGRAM_IDS`, send `/start` and tap `📊 Statistika`, or send `/stats`.
2. Verify access control by opening the same link from a non-admin Telegram account; it must show an access-denied message, and `/api/miniapp/admin/stats` must return HTTP 403.
3. The admin page should show resident households, bins and full bins, open dump and missed-pickup cases, total dump reports, route count, collected and accepted kilograms, a 7-day collection bar chart, and accepted material split.
4. With an empty pilot database, zero metrics and empty charts are expected.

## Commands

- Resident: `/start`, `/id`, `/language`, `/bins`, `/calendar`, `/missed`, `/sort`, `/impact`, `/myreports`, `/reminders`, `/reminders on`, `/reminders off`.
- Collector: `/batch`, scan QR, enter weight, `/finishbatch`.
- Recycler: `/accept ECO-YYYYMMDD-00001 12.5`.
- Admin: `/issues`, `/dumps`, `/resolve_issue ID`, `/resolve_dump ID`.

## Pilot limitations to check with the project owner

- This is a workflow pilot, not an audited municipal dispatch system. It does not yet connect to a real route optimizer, weighbridge, or recycler API.
- The schedule is shared at service level, not assigned to individual neighborhoods.
- The first registration flow creates three household containers using one address, location and photo. Confirm that this matches the actual operating model before broader rollout.
- Accepted kilograms are allocated proportionally across route pickups for the resident eco-balance. This is an estimate when the facility reports a different total weight; the per-bin weights remain available in collection events.
- The Mini App dump-report form records address, description, and coordinates but does not upload a photo. Use the Telegram bot when photographic evidence is required.
- Telegram bot messages and Telegram IDs are required for the pilot. Configure only test accounts and a test token in Railway.
- This is an initial Karakalpak/Russian pilot translation. Review the Karakalpak wording with a native speaker before public launch.
- Mini App APIs require a valid Telegram WebApp `initData` signature; opening the page in a regular browser without launching it from Telegram will not load user data.

## Quick operational checks

- Open `https://<service-domain>/` to confirm the test service page loads.
- Open `https://<service-domain>/healthz`; require HTTP 200 and status `ok` after configuring the bot token.
- Railway deployment must show a successful build and a running replica.
- Review logs for database migration errors, Telegram polling errors, and rejected test cases.
- Verify that no production token or production database URL is present in the test project.
