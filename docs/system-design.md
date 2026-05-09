# System Design — GitHub Release Notification API

> Документ описує архітектуру сервісу: що він робить, з чого складається, як рухаються дані, які компроміси прийняті й куди система може еволюціонувати. Конкретні архітектурні рішення з мотивацією — у [`adr/`](adr/).

## 1. Призначення

Сервіс дозволяє користувачу **підписатися на email‑сповіщення про нові релізи** обраного публічного GitHub‑репозиторію. Користувач підтверджує підписку через лист, після чого фоновий сканер періодично опитує GitHub API і, виявивши новий тег, надсилає лист‑сповіщення з посиланням на реліз і токеном відписки.

### Функціональні можливості

- `POST /api/subscribe` — створити підписку (`email`, `repo` у форматі `owner/name`).
- `GET /api/confirm/{token}` — підтвердити підписку.
- `GET /api/unsubscribe/{token}` — відписатися.
- `GET /api/subscriptions?email=...` — переглянути підписки конкретного email.
- gRPC‑аналоги тих самих чотирьох операцій (сервіс `subscription.SubscriptionService`).
- Фоновий сканер релізів і відсилання повідомлень.
- Метрики Prometheus, готова OpenAPI‑специфікація, статична HTML‑сторінка‑демо.

### Нефункціональні цілі

- Надійність доставки листів важливіша за «миттєвість» — допускається затримка до одного інтервалу сканування (за замовчуванням 5 хв).
- Стійкість до короткочасних збоїв GitHub/SMTP (retry + кеш).
- Економія GitHub rate‑limit (кеш `latest_release`).
- Простий локальний запуск (`docker compose up`).

## 2. Контекст системи (C4 Level 1)

```mermaid
flowchart LR
  user["Користувач<br/>(браузер / CLI / клієнт API)"]
  classroom["GitHub<br/>(REST API репозиторіїв і релізів)"]
  smtp["SMTP-сервер<br/>(MailHog у dev / AWS SES у prod)"]
  inbox["Поштова скринька<br/>користувача"]
  app["GitHub Release Notification API"]

  user -- "HTTP/REST, gRPC" --> app
  app -- "GET /repos/.../releases/latest" --> classroom
  app -- "SMTP submit" --> smtp
  smtp -- "delivery" --> inbox
  user -- "клік по лінку confirm/unsubscribe" --> app
```

## 3. Контейнери (C4 Level 2)

```mermaid
flowchart LR
  subgraph client["Клієнти"]
    browser["Браузер<br/>(статична сторінка /)"]
    api_client["HTTP / gRPC клієнти<br/>(curl, Postman, grpcurl)"]
  end

  subgraph host["Хост / docker-compose"]
    app["app<br/>Go-бінар (Gin + grpc)<br/>:8080 HTTP, :50051 gRPC"]
    pg[("PostgreSQL 16<br/>репозиторії, підписки, токени")]
    redis[("Redis 7<br/>кеш latest_release / repo_exists")]
    mh["MailHog<br/>dev SMTP :1025 + UI :8025"]
  end

  classroom["GitHub API"]
  ses["AWS SES<br/>(prod)"]

  browser -- "/api/* HTTP" --> app
  api_client -- "REST :8080 / gRPC :50051" --> app
  app <-- "SQL" --> pg
  app <-- "GET/SET TTL" --> redis
  app -- "SMTP" --> mh
  app -- "SMTP+TLS" --> ses
  app -- "HTTPS" --> classroom
```

### Внутрішні модулі (гексагональна архітектура — див. [ADR-0001](adr/0001-hexagonal-architecture.md))

```mermaid
flowchart TB
  subgraph inbound["Inbound adapters"]
    httpA["HTTP / Gin<br/>handler.go, router.go,<br/>middleware (APIKey, metrics)"]
    grpcA["gRPC server<br/>handler.go, server.go"]
  end

  subgraph application["Application (use-cases)"]
    sub["SubscriptionService<br/>Subscribe/Confirm/<br/>Unsubscribe/GetSubscriptions"]
    scan["ScannerService<br/>периодичний цикл"]
    notif["NotifierService<br/>NotifySubscribers"]
  end

  subgraph domain["Domain"]
    model["model.go<br/>Repository, Subscription, Release"]
    ports["domain/port<br/>SubscriptionRepository, GitHubClient,<br/>Cache, Mailer"]
    errs["errors.go"]
  end

  subgraph outbound["Outbound adapters"]
    pgRepo["postgres/repository.go"]
    ghClient["github/client.go<br/>+ cached_client + instrumented_client"]
    rcache["redis/cache.go"]
    mailer["email/sender.go"]
  end

  metrics["metrics<br/>Prometheus collectors"]
  platform["platform<br/>retry/backoff"]

  httpA --> sub
  grpcA --> sub
  scan --> ghClient
  scan --> pgRepo
  scan --> notif
  notif --> mailer
  notif --> pgRepo
  sub --> ports
  scan --> ports
  notif --> ports
  ports -. реалізують .-> pgRepo
  ports -. реалізують .-> ghClient
  ports -. реалізують .-> rcache
  ports -. реалізують .-> mailer
  httpA --> metrics
  ghClient --> metrics
  notif --> platform
  ghClient --> platform
```

## 4. Модель даних

Дві таблиці у PostgreSQL (див. `migrations/000001_init.up.sql`).

```mermaid
erDiagram
  REPOSITORIES ||--o{ SUBSCRIPTIONS : "1 to many"
  REPOSITORIES {
    int id PK
    varchar owner
    varchar repo
    varchar last_seen_tag "nullable"
    timestamptz created_at
    timestamptz updated_at
  }
  SUBSCRIPTIONS {
    int id PK
    varchar email
    int repository_id FK
    bool confirmed
    varchar confirm_token UK "32B hex, 64 chars"
    varchar unsubscribe_token UK
    timestamptz created_at
    timestamptz updated_at
  }
```

Унікальність та індекси:

- `UNIQUE(owner, repo)` — один запис на репозиторій.
- `UNIQUE(email, repository_id)` — повторна підписка → 409 (див. [ADR-0007](adr/0007-token-based-confirmation.md)).
- `UNIQUE(confirm_token)`, `UNIQUE(unsubscribe_token)` — одноразові, непригадувані токени.
- Індекси на `email`, `confirmed=TRUE`, на обидва токени (швидкий lookup за лінком).

## 5. Основні потоки

### 5.1. Підписка та підтвердження

```mermaid
sequenceDiagram
    autonumber
    actor U as Користувач
    participant H as HTTP handler
    participant S as SubscriptionService
    participant GH as GitHubClient (cached)
    participant DB as PostgreSQL
    participant M as Mailer (SMTP)

    U->>H: POST /api/subscribe {email, repo}
    H->>S: Subscribe(email, repo)
    S->>GH: RepoExists(owner, repo)
    GH-->>S: true / false
    alt repo не існує
        S-->>H: ErrRepoNotFound (404)
    else існує
        S->>DB: upsert repository, lookup subscription
        alt вже є для цього email+repo
            S-->>H: ErrAlreadySubscribed (409)
        else нова
            S->>DB: insert subscription (confirm_token, unsubscribe_token)
            S->>M: SendConfirmation(email, link={base}/api/confirm/{token})
            M-->>S: ok
            S-->>H: ok (200)
            U->>U: відкриває лист
            U->>H: GET /api/confirm/{token}
            H->>S: Confirm(token)
            S->>DB: set confirmed=true
            S-->>H: ok (200)
        end
    end
```

Якщо лист не відправився — підписка **відкочується** (`DeleteSubscription`) і клієнт отримує 5xx. Це гарантує, що в БД немає «зомбі‑підписок» без листа.

### 5.2. Цикл сканера релізів

```mermaid
sequenceDiagram
    autonumber
    participant T as time.Ticker (SCANNER_INTERVAL=5m)
    participant SC as ScannerService
    participant DB as PostgreSQL
    participant GH as GitHubClient (cached)
    participant N as NotifierService
    participant M as Mailer

    loop кожен інтервал
        T->>SC: tick
        SC->>DB: GetRepositoriesWithConfirmedSubs()
        loop для кожного repo
            SC->>GH: GetLatestRelease(owner, repo)
            GH->>GH: Redis GET (TTL=10m)
            alt cache hit
                GH-->>SC: release{tag}
            else miss
                GH->>GH: HTTP GET /releases/latest
                GH->>GH: Redis SET TTL
                GH-->>SC: release{tag}
            end
            alt tag == repo.last_seen_tag
                SC->>SC: skip
            else last_seen_tag == ""
                SC->>DB: UpdateLastSeenTag(release.tag)<br/>(baseline, без розсилки)
            else новий тег
                SC->>N: NotifySubscribers(repo, release)
                N->>DB: GetConfirmedSubscriptionsByRepoID
                loop кожен підписник
                    N->>M: SendReleaseNotification(...) (з retry+backoff)
                end
                SC->>DB: UpdateLastSeenTag(release.tag)
            end
        end
    end
```

Особливості:

- **Сканер опитує GitHub лише для тих репо, де є confirmed‑підписки** — менше зайвих запитів.
- **Перший виявлений реліз** не розсилається: він стає baseline (інакше при новій підписці на репо з історією прилетіли б «фейкові» сповіщення).
- **Кеш `latest_release`** з TTL зменшує частоту звернень до GitHub; компроміс — затримка виявлення нового тегу до `SCANNER_INTERVAL + GITHUB_CACHE_TTL` (див. [ADR-0004](adr/0004-redis-cache-for-github.md)).
- Якщо Redis недоступний — лог‑warning, кеш вимикається, прямі HTTP‑дзвінки до GitHub.
- Email‑відправка обернена в експоненційний backoff (`platform.RetryConfig`, transient errors only).

### 5.3. Відписка

```mermaid
sequenceDiagram
    actor U as Користувач
    participant H as HTTP handler
    participant S as SubscriptionService
    participant DB as PostgreSQL

    U->>H: GET /api/unsubscribe/{token}
    H->>S: Unsubscribe(token)
    S->>DB: GetSubscriptionByUnsubscribeToken
    alt не знайдено
        S-->>H: ErrTokenNotFound (404)
    else
        S->>DB: DeleteSubscription
        S-->>H: ok (200)
    end
```

## 6. Конфігурація і середовища

Всі параметри керуються через ENV (див. `internal/config/config.go`):

| Група    | Ключі (приклади)                                                   | За замовчуванням                                       |
| -------- | ------------------------------------------------------------------ | ------------------------------------------------------ |
| Server   | `SERVER_PORT`, `*_TIMEOUT`                                         | `8080`, 15s/15s/60s                                    |
| DB       | `DB_HOST/PORT/USER/PASSWORD/NAME`, `DB_QUERY_TIMEOUT`              | localhost/5432/postgres/postgres/releases, 5s          |
| Redis    | `REDIS_ADDR`, `REDIS_DB`, `REDIS_*_TIMEOUT`                        | localhost:6379                                         |
| SMTP     | `SMTP_HOST/PORT/USER/PASSWORD/FROM/TIMEOUT`                        | localhost:1025 (MailHog у docker-compose)              |
| GitHub   | `GITHUB_TOKEN`, `GITHUB_BASE_URL`, `GITHUB_CACHE_TTL`              | `https://api.github.com`, 10m                          |
| Scanner  | `SCANNER_INTERVAL`, `SCANNER_CYCLE_TIMEOUT`, `SCANNER_REPO_TIMEOUT`| 5m, 4m, 30s                                            |
| gRPC     | `GRPC_PORT`                                                        | 50051                                                  |
| Auth     | `API_KEY`                                                          | пусто (опційний middleware)                            |
| Logging  | `LOG_LEVEL`                                                        | info (slog JSON)                                       |

Середовища:

- **Local dev** — `docker-compose.yml` піднімає `app + postgres + redis + mailhog`. `SMTP_USER/PASSWORD` примусово порожні, бо `net/smtp` Go не дозволяє `PLAIN AUTH` по нешифрованому з’єднанню.
- **Prod** — `docker-compose.prod.yml`, реальні секрети через ENV‑файл або менеджер секретів. Інфраструктура AWS — у `terraform/` (VPC, RDS PostgreSQL, ElastiCache Redis, EC2, SES — за потреби).

## 7. Спостережуваність

- **Логи** — `slog` у JSON, рівень керується `LOG_LEVEL`.
- **Метрики Prometheus** на `/metrics`:
  - `http_requests_total{method,path,status}`, `http_request_duration_seconds`,
  - `github_api_calls_total{endpoint,status}`,
  - `emails_sent_total{type,status}`,
  - `active_subscriptions`, `scan_cycles_total`.
- **Health‑check** — фактично HTTP `200 /` достатньо для поточних потреб; за потреби додасться `/healthz` із пінгом БД/Redis.

## 8. Безпека

- **Автентифікація API (опційна).** Якщо `API_KEY` непорожній, middleware `APIKeyAuth` вимагає заголовок `X-API-Key`. Без значення — публічні ендпоінти (для веб‑демо).
- **Токени.** `confirm_token` і `unsubscribe_token` — 32 байти з `crypto/rand`, hex‑encoded (64 символи), `UNIQUE` у БД, не вгадуються brute‑force за прийнятний час.
- **GitHub Token (опційний).** Без токена rate‑limit різко обмежений; токен передається лише в `Authorization: Bearer ...` до `api.github.com`.
- **SMTP.** У prod очікується TLS (порт 587 + STARTTLS); код вмикає `STARTTLS` якщо сервер його повідомляє.
- **Postgres / Redis.** У docker‑compose не виставлені назовні без потреби; у prod — у приватній мережі (Terraform VPC).
- **CORS.** Не налаштовано (єдиний клієнт — статична сторінка з того самого хоста). Якщо знадобиться зовнішній фронт — додати middleware.

## 9. Надійність і стійкість

- **Retry з експоненційним backoff** (`internal/platform/retry.go`) для:
  - HTTP‑дзвінків до GitHub (`max_retries=3`, jitter ±25%);
  - SMTP‑відправки (тільки transient errors: `dial`, `timeout`, `EOF`, `reset`, …).
- **Кеш як graceful degradation:** Redis недоступний → попередження в лог, прямий HTTP без падіння.
- **Контексти і таймаути** скрізь: `cycleCtx` для всього циклу сканера, `repoCtx` на репо, окремі таймаути на DB‑запит, SMTP, HTTP.
- **Graceful shutdown:** `SIGINT/SIGTERM` → `context.Cancel` для сканера, `httpServer.Shutdown`, `grpcServer.GracefulStop`, ліміт `SHUTDOWN_TIMEOUT`.

## 10. Ризики й майбутня еволюція

| Ризик / обмеження                                                    | Поточна позиція                                              | Можливий розвиток                                                    |
| -------------------------------------------------------------------- | ------------------------------------------------------------ | -------------------------------------------------------------------- |
| Polling замість push → затримка виявлення релізу                     | Допустима (5–15 хв, див. [ADR-0002](adr/0002-polling-vs-webhooks.md)) | GitHub Webhooks для public репо з підпискою «releases»; require Webhook secret |
| Один інстанс сканера → всі цикли в одному процесі                    | OK поки число confirmed‑repos невелике                       | Виносити сканер у окремий деплой; шардинг репозиторіїв; черга задач  |
| Email‑відправка синхронно у циклі сканера                            | Простіше, retry на місці                                     | Винести в чергу (NATS/Kafka/RabbitMQ); окремий worker для розсилки   |
| Rate‑limit GitHub                                                    | Кеш + опційний токен                                         | Conditional requests (ETag / `If-None-Match`); GraphQL для bulk      |
| Кеш «застарілий» = пізніше сповіщення                                | Свідомий компроміс (TTL 10 хв)                               | Зменшити TTL для популярних репо; інвалідація після виявлення        |
| Немає health/readiness endpoints                                     | Достатньо для compose                                        | Додати `/healthz`, `/readyz` з пінгом залежностей                    |
| Один регіон/моноліт                                                  | Простота                                                     | Multi‑region deploy, RDS read replica, distributed cache             |

---

Останні оновлення: див. історію Git у `docs/`.
