# EcoMap Bin4Home pilot: test guide

This branch adds a pilot workflow for household recycling, collection operations, and a Telegram Mini App. It is isolated from the production branch and database. The Mini App URL is `https://ecomap-bot-test-pilot.up.railway.app/miniapp/`; it must be launched from an existing Telegram Web App button so Telegram can provide signed user data. Karakalpak is the default language, with Russian available from the language toggle. Residents can report a missed pickup, report an illegal dump with location, and manage reminders directly in the Mini App.

## Before the first Telegram test

1. The test Railway service has `BOT_TOKEN` and `ADMIN_TELEGRAM_IDS` configured. Keep their values private.
2. `BOT_POLLING_ENABLED=false` in the pilot. This is required because the live bot uses the same Telegram token; Telegram allows only one `getUpdates` poller. Do not enable polling in the pilot while the live bot is running.
3. Open the EcoMap Mini App from its existing Telegram Web App button. A normal browser visit is not authenticated and will not load resident data.
4. `PICKUP_SCHEDULE` is set to a demo schedule (`mon:plastic,tue:paper,fri:glass`) with `TIMEZONE=Asia/Tashkent`.
5. Check `https://ecomap-bot-test-pilot.up.railway.app/healthz`; expected response is `{"status":"ok"}`. The pilot Mini App page should return HTTP 200.

`DATABASE_URL` should be wired to the separate Postgres service. The service applies additive schema migrations during startup. Do not point this test service at production data.

## Resident scenarios

| Scenario | Steps | Expected result |
|---|---|---|
| Register a household | Use an existing bot's registration flow | Three QR codes arrive, one each for plastic, paper and glass; the Mini App lists them when opened from Telegram. |
| Open the Mini App | Tap the existing `EcoMap Mini App` Web App button in the bot chat | A responsive dashboard opens inside Telegram. Karakalpak is selected by default; card transitions and animated charts respect reduced-motion settings. |
| Switch the language | Use the Mini App language toggle | Russian and Karakalpak can be selected and the preference is saved for that Telegram account. |
| See the pickup schedule | Open the Mini App dashboard | Configured weekdays and material types appear in the selected language. |
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

- The test service itself serves the Mini App APIs only; bot commands are handled by the one currently active bot poller. Use a dedicated test token before testing separate command handlers end-to-end.

## Pilot limitations to check with the project owner

- This is a workflow pilot, not an audited municipal dispatch system. It does not yet connect to a real route optimizer, weighbridge, or recycler API.
- The schedule is shared at service level, not assigned to individual neighborhoods.
- The first registration flow creates three household containers using one address, location and photo. Confirm that this matches the actual operating model before broader rollout.
- Accepted kilograms are allocated proportionally across route pickups for the resident eco-balance. This is an estimate when the facility reports a different total weight; the per-bin weights remain available in collection events.
- The Mini App dump-report form records address, description, and coordinates but does not upload a photo. Use the Telegram bot when photographic evidence is required.
- The pilot service retains the configured token for Telegram Web App authentication but has polling disabled to avoid competing with the live bot. Use a separate test token to run a second bot poller and test bot commands independently.
- This is an initial Karakalpak/Russian pilot translation. Review the Karakalpak wording with a native speaker before public launch.
- Mini App APIs require a valid Telegram WebApp `initData` signature; opening the page in a regular browser without launching it from Telegram will not load user data.

## Quick operational checks

- Open `https://<service-domain>/` to confirm the test service page loads.
- Open `https://<service-domain>/healthz`; require HTTP 200 and status `ok` after configuring the bot token.
- Railway deployment must show a successful build and a running replica.
- Review logs for database migration errors, Telegram polling errors, and rejected test cases.
- Verify that no production token or production database URL is present in the test project.
