# GitHub Release Notification API

REST and gRPC service for subscribing to **email notifications** when a **GitHub repository** publishes a new release. Users subscribe with an email and repo name, confirm via email, and can unsubscribe with a token. A background scanner compares the latest release tag with the last seen tag and notifies subscribers.

### Deployed project

**Public URL:** [http://51.102.160.14](http://51.102.160.14) — HTTP (HTML and REST behind Nginx on port 80). **gRPC:** `51.102.160.14:50051`.

*(Infrastructure: AWS EC2 + RDS + ElastiCache; see [Deploy](#deploy-aws--github-actions).)*

## Features

- **Subscribe** — Register an email for a GitHub repo (`owner/name`); validates the repo via GitHub API.
- **Email confirmation** — Subscription is inactive until the user confirms via link.
- **Unsubscribe** — One-click unsubscribe using a token from emails.
- **List subscriptions** — Query subscriptions by email.
- **Release scanner** — Periodic job detects new tags/releases and sends notifications.
- **Resilience** — Redis caching for GitHub API, retries and rate-limit handling, timeouts on all outbound calls.
- **Metrics** — Prometheus endpoint `/metrics` with HTTP, GitHub, email, scanner, and subscription gauges/counters (details under **`GET /metrics`** below).
- **Optional gRPC** — Same subscription flows over gRPC (port `GRPC_PORT`, default `50051`).
- **Optional API key** — Set `API_KEY` to require `X-API-Key` on `/api/*` (the bundled HTML page does not send it).
- **Automated tests** — **Unit tests** on core business logic (`internal/application`, `internal/platform`); **integration-style tests** for the HTTP API and GitHub client using `httptest` (see [Testing](#testing)).

## Release detection logic

The **scanner** runs on a fixed interval (`SCANNER_INTERVAL`). Each cycle it loads repositories that have **at least one confirmed subscription**, then for each repo fetches the **latest GitHub release** (tag name).

For every repository row the database stores **`last_seen_tag`**:

- **First time a release exists** and `last_seen_tag` is empty: the service treats that tag as a **baseline** — it only **updates** `last_seen_tag` in the database. **No notification email is sent**, so existing releases do not spam subscribers when they first connect.
- **Later**, when the latest release tag **differs** from `last_seen_tag`: subscribers are **notified**, then `last_seen_tag` is updated to that tag.
- If the latest tag **matches** `last_seen_tag`, nothing happens.

So emails go out only when a **new** release tag appears **after** the baseline was established.

## API endpoints

Base path for REST: **`/api`**. Unless `API_KEY` is set, JSON endpoints do not require a header.

### 1. `POST /api/subscribe`

- **Description**: Subscribe an email to release notifications for a repository.
- **Content-Type**: `application/json`
- **Body**:
  - `email` (string, required) — Subscriber email.
  - `repo` (string, required) — GitHub repository in `owner/repo` form.
- **Responses**:
  - `200 OK` — Subscription recorded; confirmation email sent.
  - `400 Bad Request` — Invalid body, email, or repo format.
  - `404 Not Found` — Repository not found on GitHub.
  - `409 Conflict` — Already subscribed.

### 2. `GET /api/confirm/{token}`

- **Description**: Confirm a subscription using the token from the confirmation email.
- **Path parameters**:
  - `token` (required) — Confirmation token.
- **Responses**:
  - `200 OK` — Confirmed.
  - `400 Bad Request` — Invalid token.
  - `404 Not Found` — Token not found.

### 3. `GET /api/unsubscribe/{token}`

- **Description**: Unsubscribe using the token from notification emails.
- **Path parameters**:
  - `token` (required) — Unsubscribe token.
- **Responses**:
  - `200 OK` — Unsubscribed.
  - `400 Bad Request` — Invalid token.
  - `404 Not Found` — Token not found.

### 4. `GET /api/subscriptions`

- **Description**: Returns **all current subscriptions** for the given email — rows that still exist in the database (unsubscribe removes a row, so it will not appear).
- **Query parameters**:
  - `email` (required) — Email address.
- **Response body**: JSON array of objects with `email`, `repo` (full name), `confirmed` (whether the user completed email confirmation), and `last_seen_tag` when applicable.
- **Responses**:
  - `200 OK` — JSON array (may be empty).
  - `400 Bad Request` — Missing `email`.

### 5. `GET /metrics`

- **Description**: Prometheus **text exposition** format for scraping or manual checks. Path is **not** under `/api`; API key middleware does not apply.
- **Service-specific metrics** (from `internal/metrics`):
  - `http_requests_total`, `http_request_duration_seconds` — request counts and latency by method, path, and status.
  - `github_api_calls_total` — outbound GitHub API calls by endpoint and status.
  - `emails_sent_total` — email send attempts by type and status.
  - `active_subscriptions` — gauge of confirmed subscriptions.
  - `scan_cycles_total` — counter of completed scanner cycles.
- **Also exposed**: default **Go runtime** and **process** series (`go_*`, `process_*`, etc.) from the Prometheus Go client.

### 6. `GET /`

- **Description**: Static HTML page for subscription (served from `web/`).

## OpenAPI specification

The contract is [`api/swagger.yaml`](api/swagger.yaml) (Swagger 2.0). **`host` is omitted** on purpose: clients resolve the API against the **same origin** you use to fetch the spec (aligned with **`BASE_URL`** per environment).

While the process is running, the spec is also exposed over HTTP (**no** `X-API-Key` required):

| Environment | URL |
|-------------|-----|
| Local (Compose) | `http://localhost:8080/api/swagger.yaml` |
| Deployed (example) | `http://51.102.160.14/api/swagger.yaml` |

Import that URL in **Postman** or **Swagger Editor** to generate requests against that host. There is **no built-in Swagger UI** in this repo; use Postman, editor.swagger.io, or a desktop client.

## Architecture

**Hexagonal (ports & adapters)**:

- `internal/domain` — Entities and port interfaces.
- `internal/application` — Subscription, scanner, notifier services.
- `internal/adapter/inbound` — HTTP (Gin), gRPC.
- `internal/adapter/outbound` — PostgreSQL, GitHub API, SMTP, Redis cache.

Migrations run automatically on application startup (`golang-migrate`).

## Setup

### Prerequisites

- **Go** 1.25+
- **Docker** and **Docker Compose** (recommended for local stack)

### Running locally (Docker Compose)

1. Clone the repository:

```bash
git clone https://github.com/l4ndm1nes/GitHub-Release-Notification-API.git
cd GitHub-Release-Notification-API
```

2. Configure environment:

```bash
cp .env.example .env
```

3. Edit `.env` — important values:

| Variable | Purpose |
|----------|---------|
| `GITHUB_TOKEN` | Strongly recommended for GitHub API rate limits (5000 req/h vs 60 without token). |
| `DB_*` | Defaults match Compose service names (`postgres`, port `5432`). |
| `REDIS_ADDR` | Default `redis:6379` inside Compose. |
| `SMTP_*` | Point to **MailHog** in Compose: host `mailhog`, port `1025` (see `.env.example`). |
| `BASE_URL` | e.g. `http://localhost:8080` for local links in emails. |

4. Start the stack:

```bash
docker-compose up --build
```

5. Open **http://localhost:8080** — HTML form. **MailHog UI**: http://localhost:8025 — to read outgoing mail.

### Amazon SES (production email)

If you use **Amazon SES** instead of MailHog, new accounts are usually in the **SES sandbox**: you can send mail only to **verified recipient addresses**, and the **sender identity** (from-address / domain) must also be **verified**. For local or demo testing with SES, point `SMTP_*` and `SMTP_FROM` at verified identities so both sides are allowed.

To deliver to **arbitrary recipient addresses**, request **production access** (move out of the sandbox) in the SES console. Until then, SES will not deliver to unverified recipients, so confirmation and notification mail to random addresses may never arrive.

### Running locally (Go only)

Start Postgres, Redis, and MailHog (or compatible SMTP), then:

```bash
go mod download
export DB_HOST=localhost REDIS_ADDR=localhost:6379 SMTP_HOST=localhost
# align .env or exports with your ports
go run ./cmd/server
```

## Environment variables

Full list and defaults: **[`.env.example`](.env.example)**. For production `.env` on the server, start from **[`deploy/ec2.env.example`](deploy/ec2.env.example)**.

## Testing

**Unit tests** cover **business logic** in `internal/application` (subscription lifecycle, scanner baselines and notifications, notifier paths, error handling) and helpers in `internal/platform`, using **in-memory mocks** for repositories, GitHub, and mail — fast, deterministic, no Docker required for these packages.

**Integration-style tests** (broader than single functions): **[`internal/adapter/inbound/http`](internal/adapter/inbound/http)** runs the real **Gin** router and handlers against `httptest` with mocked application dependencies; **[`internal/adapter/outbound/github`](internal/adapter/outbound/github)** drives the HTTP client against a **local `httptest` server** (retries, 429/5xx, timeouts). These verify wiring and real HTTP request/response behavior. There is **no** separate suite against a live PostgreSQL — persistence is mocked at the application layer.

```bash
go test ./...
go test -v -race ./...    # as in CI (Linux runners)
```

## Development commands

There is no Makefile; use standard tooling:

```bash
# Format
go fmt ./...

# Coverage
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Lint (requires golangci-lint installed)
golangci-lint run
```

CI runs `golangci-lint` and `go test -race` on every push and pull request (see `.github/workflows/ci.yml`).

## Deploy (AWS + GitHub Actions)

1. **Infrastructure** — Terraform under [`terraform/`](terraform/) (VPC, RDS, ElastiCache, EC2, SES-related resources). Configure variables (e.g. `terraform.tfvars`) and apply with your remote state backend.
2. **Application** — Production Compose file: [`docker-compose.prod.yml`](docker-compose.prod.yml) (single `app` container; DB/Redis external).
3. **Secrets on GitHub** (repository **Settings → Secrets and variables → Actions**): `EC2_HOST`, `EC2_USER`, `EC2_SSH_PRIVATE_KEY` (full PEM); optional `EC2_DEPLOY_PATH`, `EC2_COMPOSE_FILE`.
4. **`.env` on the server** — Place next to `docker-compose.prod.yml` (not committed). Use [`deploy/ec2.env.example`](deploy/ec2.env.example) as a template.

Pushing to **`main`** triggers lint, tests, Docker image build, and optional deploy over SSH.

## Testing with curl

With default local setup and **no** `API_KEY`:

### Subscribe (JSON)

```bash
curl -s -X POST "http://localhost:8080/api/subscribe" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"user@example.com\",\"repo\":\"octocat/Hello-World\"}"
```

### Confirm

```bash
curl -s "http://localhost:8080/api/confirm/<token-from-email>"
```

### Unsubscribe

```bash
curl -s "http://localhost:8080/api/unsubscribe/<token-from-email>"
```

### List subscriptions

```bash
curl -s "http://localhost:8080/api/subscriptions?email=user@example.com"
```

If `API_KEY` is set, add:

`-H "X-API-Key: <your-key>"`

## Project structure

```text
GitHub-Release-Notification-API/
├── api/
│   ├── proto/                 # buf.yaml, subscription.proto
│   └── swagger.yaml           # OpenAPI 2.0 (also served at GET /api/swagger.yaml)
├── cmd/server/                # Application entrypoint (main)
├── deploy/
│   └── ec2.env.example        # Production .env template (copy to server; deploy/ec2.env is gitignored)
├── internal/
│   ├── adapter/inbound/       # HTTP (Gin), gRPC; grpc/pb/ — generated protobuf code
│   ├── adapter/outbound/      # Postgres, GitHub, email, Redis
│   ├── application/           # Subscription, scanner, notifier services
│   ├── config/
│   ├── domain/                # Models; domain/port/ — interfaces
│   ├── metrics/
│   └── platform/              # Retry helpers
├── migrations/                # SQL migrations (golang-migrate)
├── terraform/                 # AWS infra (apply locally; not run in GitHub Actions)
├── web/                       # Static HTML for /
├── .github/workflows/         # CI/CD (lint, test, build, Docker, optional deploy)
├── .env.example               # Environment template (copy to .env locally)
├── .golangci.yml              # Linter config used in CI
├── buf.gen.yaml               # buf codegen for gRPC stubs
├── docker-compose.yml         # Local dev stack
├── docker-compose.prod.yml    # Production (app container only)
├── Dockerfile
├── go.mod
├── go.sum
└── README.md
```
