# Telegram Bot Template

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Telegram Bot API](https://img.shields.io/badge/Telegram%20Bot%20API-v5-0088cc?style=flat&logo=telegram)](https://core.telegram.org/bots/api)

A production-ready Telegram bot template in Go for order intake, payment moderation, and team
workflow through a kanban board built on forum topics.

**Highlights**
- A multi-step intake form with deadline parsing, volume validation, and file uploads
- A channel-subscription gate, with an optional bypass for admins
- A kanban board on forum topics: orders move automatically through "In progress", "Paid",
  "Done" columns
- Receipt moderation with role-based access: accepting payment, entering the amount, moving
  cards, and notifying the user
- Team notifications: weekly reports, deadline reminders, fast search, and CSV/PDF export

## Stack
- Go 1.25.x
- [go-telegram-bot-api v5](https://github.com/go-telegram-bot-api/telegram-bot-api)
- PostgreSQL via [`lib/pq`](https://github.com/lib/pq)
- [`gofpdf`](https://github.com/jung-kurt/gofpdf) for PDF export

## Project layout
```
tgbot/
├── cmd/bot/main.go            # entry point, getUpdates/webhook modes, starts the scheduler
├── internal/
│   ├── config/                # environment variable loading
│   ├── constants/             # order status enums
│   ├── handlers/              # all Telegram routing and business logic
│   │   ├── router.go          # form FSM, subscription gate, update dispatch
│   │   ├── commands.go        # /start, /profile, /help, /analytics, /find, /export, /clear_db
│   │   ├── callbacks.go       # inline buttons, pagination, admin inline mode
│   │   ├── board.go           # forum-topic kanban board
│   │   ├── receipt.go         # file upload and validation
│   │   ├── payment.go         # payment confirmation, status transitions
│   │   ├── analytics.go       # reports, CSV/PDF export
│   │   ├── notifications.go   # weekly reports, deadline reminders
│   │   └── admin.go           # admin-only helper commands
│   ├── logger/                # shared request/error log format
│   ├── parsing/               # date and relative-deadline parsing
│   ├── scheduler/             # periodic job scheduler
│   ├── state/                 # FSM sessions, /start tracking
│   ├── storage/               # PostgreSQL access, migrations, search, analytics
│   └── validation/             # receipt file constraints
├── env.example                # config template
├── Makefile                   # build, run, Docker helpers
├── Dockerfile
├── data/                       # container DB directory (created automatically)
└── README.md
```

## Quick start

```bash
git clone https://github.com/whiterage/go-telegram-bot-template.git
cd go-telegram-bot-template

cp env.example .env
# fill in BOT_TOKEN, CHANNEL_ID, ADMIN_IDS, etc.

make docker-build
make docker-run

# or locally
make run
```

## Environment setup
1. Install Go ≥ 1.25 and `make`.
2. Create `.env` from the example: `cp env.example .env`.
3. Fill in the required variables below — the bot won't start without them.

### Environment variables
| Variable | Required | Description |
|---|---|---|
| `BOT_TOKEN` | yes | Token from @BotFather |
| `APP_ENV` | no | `dev` enables debug logging for the Telegram API |
| `CHANNEL_ID` | yes | Channel ID (`-100…` format) for the subscription check |
| `CHANNEL_URL` | yes | Channel link for the subscribe button |
| `ADMIN_IDS` | yes | Comma-separated admin Telegram IDs |
| `ALLOW_ADMINS_BYPASS` | no | `true` by default: channel admins skip the subscription gate |
| `BOARD_CHAT_ID` | yes | Supergroup ID with the forum-topic kanban board |
| `INPROGRESS_TOPIC_ID` | yes | "In progress" topic ID |
| `PAID_TOPIC_ID` | yes | "Paid" topic ID |
| `DONE_TOPIC_ID` | yes | "Done" topic ID |
| `DEADLINE_TOPIC_ID` | yes | Topic ID for deadline digests |
| `USE_WEBHOOK` | no | `true` for webhook mode instead of getUpdates |
| `WEBHOOK_URL` | if `USE_WEBHOOK=true` | Public URL to your HTTP server |
| `WEBHOOK_ADDR` | no | Local HTTP server address (default `:8080`) |
| `WEBHOOK_PATH` | no | Webhook path (default `/telegram/webhook`) |
| `WEBHOOK_SECRET` | no | Reserved for a future webhook secret (not yet supported by API v5.5) |
| `DB_HOST` | yes | PostgreSQL host |
| `DB_PORT` | yes | PostgreSQL port |
| `DB_USER` | yes | PostgreSQL user |
| `DB_PASSWORD` | yes | PostgreSQL password |
| `DB_NAME` | yes | Database name |
| `DB_SSLMODE` | yes | SSL mode (disable/require/verify-full) |

## Running it
### Locally (getUpdates)
```bash
make run          # go mod tidy + go run ./cmd/bot
# or manually
GOFLAGS=-mod=mod go run ./cmd/bot
```

On `Ctrl+C` the bot stops fetching updates and shuts the scheduler down cleanly.

### Docker
```bash
make docker-build             # builds the tgbot:latest image
make docker-run               # getUpdates mode
make docker-run-webhook       # webhook mode, exposes port 8080
```
The container expects a `.env` file and a mounted `./data` directory for the persistent database.

## Makefile targets
| Target | Does |
|---|---|
| `make run` | Install deps and run the bot (dev) |
| `make build` | Build the `bin/bot` binary |
| `make clean` | Remove `bin/` and temp files |
| `make clean-db` | Delete the local database (dev only) |
| `make lint` | `go vet` + `gofmt -s -w .` |
| `make docker-build` | Build the Docker image |
| `make docker-run` | Run in a container, no webhook |
| `make docker-run-webhook` | Run in a container with port 8080 exposed |
| `make deploy` | (Server) `git pull` + `docker compose pull/build/up` via `scripts/deploy.sh` |

### Automated deploy (`make deploy`)
1. Clone the repo onto the server (e.g. `/opt/tgbot/app`), create `.env` and a `data/` directory.
2. Make sure Docker and the `docker compose` plugin are installed.
3. Make the script executable once: `chmod +x scripts/deploy.sh`.
4. Deploy with one command:
   ```bash
   cd /opt/tgbot/app
   APP_DIR=/opt/tgbot/app make deploy
   ```

The script runs `git fetch && git pull --rebase --autostash`, then `docker compose pull`,
`docker compose build`, and `docker compose up -d --force-recreate --remove-orphans` — `.env` and
the `data/` directory are left untouched.

## Bot commands and flows
**For users**
- `/start` — launches the intake form: venue type → category → deadline (dates, "1.5 weeks",
  "not sure") → service type → volume → theme and requirements → attachments → confirmation.
- `/profile` — a paginated list of orders, 5 at a time, with buttons to upload a receipt or check
  status.
- `/help` — a quick reference and support link.
- The "💳 Receipt" button after payment starts a file upload; PDF/JPG/PNG up to 20 MB.

**For admins** (ID must be in `ADMIN_IDS`)
- `/analytics [week|month|year|total]` — aggregated stats and conversion for the period.
- `/export [month|year|total]` — exports the current slice to `export_*.csv` and `export_*.pdf`.
- `/find <query>` — fast search by order number, theme, or category, with reply/status buttons.
- `/ratelimit` — rate-limiting stats (active users, current configuration).
- Inline mode: type `@yourbot <phrase>` in any chat to get a list of matching orders with quick
  action buttons.
- `/clear_db` — wipes the database for testing (deletes every order — use carefully).
- Board inline buttons:
  - "✅ Accept #…" — asks for the payment amount, moves the card to "Paid", notifies the client.
  - "❌ Reject #…" — sends the card back to "In progress".
  - "✅ Complete #…" / "❌ Reject #…" in the "Paid" column — final status, moves to "Done".

## Automation
- **Deadline reminders.** Every 6 hours the bot collects orders due today, tomorrow, or in 3
  days, and posts a digest to the `DEADLINE_TOPIC_ID` topic (falling back to admin DMs if that
  topic is unreachable).
- **Weekly reports.** Every Monday at 09:00, admins get a report on the past week: intake,
  payments, conversion, revenue, and refunds.

## Data
- Orders live in a single PostgreSQL `orders` table. Schema migrations are versioned; indexes for
  frequent queries are created automatically on first run.
- Order fields: user, chat, deadline, volume, notes, status, payment details, and the card's
  position on the board.
- Every status change updates the card's HTML in its board topic and is logged.
- `storage.SaveReceipt` stores the Telegram `file_id` for receipts, so files are served back by
  Telegram without extra disk usage.

## Logging and resilience
- A shared logger (`internal/logger`) tags send errors, API-request errors, and order/board
  operations.
- The subscription check uses `getChatMember`; channel admins get the bypass.
- Double-moderation guard: every callback checks the current status first and replies "Already
  processed" on a repeated press.
- Receipt file-type and size limits (20 MB) block accidental attachments.

## Rate limiting
A token-bucket rate limiter protects the bot from abuse and spam.

| Action | Limit |
| --- | --- |
| Messages | 100 per 15 minutes |
| File uploads | 20 per 20 minutes |
| Button presses | 200 per 15 minutes |
| Commands | 20 per 10 minutes |

- Tokens replenish automatically over time.
- Admins are exempt from all limits.
- Error messages are tailored to the action that got throttled.
- Inactive users are cleared from memory automatically.
- Enforced at the router level, so every update is checked without per-handler wiring.

Admins can inspect current state with `/ratelimit`: active users per action type, the configured
limits, and overall system status. More detail in
[`internal/ratelimit/README.md`](internal/ratelimit/README.md).

## Testing and demo setup
- `/clear_db` (admin-only) clears all orders.
- To reset the project entirely, delete the database (or the `./data` directory under Docker) —
  the bot recreates the schema from scratch on next start.
- For testing the subscription flow, create a test channel/group and point `.env` at its IDs.

Once `.env` is filled in and the bot is running, it's ready to take orders and keep the team's
kanban board in sync.

## License

MIT — see [LICENSE](LICENSE).

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push the branch (`git push origin feature/amazing-feature`)
5. Open a pull request

## Support

Questions or issues — open one on the
[repository's issue tracker](https://github.com/whiterage/go-telegram-bot-template/issues).
