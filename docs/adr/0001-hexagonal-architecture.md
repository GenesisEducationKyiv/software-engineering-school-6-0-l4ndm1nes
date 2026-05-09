# ADR-0001: Гексагональна архітектура (Ports & Adapters)

- **Статус:** Прийнято
- **Дата:** 2025-04-12
- **Дотичні ADR:** [0006](0006-http-and-grpc-transports.md), [0003](0003-postgres-as-primary-store.md), [0004](0004-redis-cache-for-github.md), [0005](0005-smtp-email-delivery.md)

## Контекст

Сервіс має кілька зовнішніх інтеграцій (PostgreSQL, Redis, GitHub HTTP API, SMTP) і два транспорти (HTTP та gRPC). Бізнес‑правила (валідація email/repo, життєвий цикл підписки, генерація токенів, цикл сканування релізів) — однакові незалежно від транспорту чи конкретної реалізації сховища. Без чіткого поділу шарів ці правила швидко «розпливуться» по handler‑ах і репозиторіях, тести стануть тендітними, а заміна, наприклад, сховища — болючою.

## Розглянуті варіанти

1. **Гексагональна архітектура (Hexagonal / Ports & Adapters).** Domain і use‑case у центрі; інтерфейси (`port.SubscriptionRepository`, `port.GitHubClient`, `port.Cache`, `port.Mailer`) — порти; реалізації (Postgres, Redis, HTTP‑клієнт, SMTP) — адаптери; HTTP/gRPC — теж адаптери (inbound).
2. **Класичний MVC‑шар «handler → service → repository»** без явних портів — хендлери залежать прямо від конкретних типів.
3. **Clean architecture з більш суворими шарами (entities/usecases/interfaces)** — ближча до Hexagonal, але з помітно більшою кількістю абстракцій.

## Рішення

Обрана **гексагональна архітектура**.

- `internal/domain` — тільки моделі, бізнес‑помилки, без зовнішніх імпортів.
- `internal/domain/port` — інтерфейси (порти).
- `internal/application` — use‑cases (`SubscriptionService`, `ScannerService`, `NotifierService`), залежать **лише** від `port.*`.
- `internal/adapter/inbound/...` — HTTP та gRPC адаптери.
- `internal/adapter/outbound/...` — Postgres, Redis, GitHub, email.
- `cmd/server/main.go` — composition root.

## Наслідки

**Плюси**

- Бізнес‑логіка тестується юніт‑тестами з простими mock‑ами (`mockRepo`, `mockGitHub`, `mockMailer`) — швидкі тести без БД/мережі.
- Додавання другого транспорту (gRPC) звелось до нового inbound‑адаптера без змін у `application/*` (див. [ADR-0006](0006-http-and-grpc-transports.md)).
- Cache, instrumentation і retry додаються через **декоратори** портів: `cached_client.go`, `instrumented_client.go` обгортають базовий `GitHubClient` без зміни виклику.
- Можна без болю замінити PostgreSQL на іншу СКБД — переписати лише адаптер.

**Мінуси / трейдофи**

- Більше типів та інтерфейсів, ніж у «прямому» підході — для дуже маленького проєкту виглядає надмірно.
- Потрібна дисципліна: не імпортувати конкретні адаптери з `application/*`.
- Деяка дублікація DTO/мапінгу між `domain` і HTTP/gRPC контрактами.
