# ADR-0006: HTTP (REST) і gRPC як два транспортних адаптери

- **Статус:** Прийнято
- **Дата:** 2025-04-12
- **Дотичні ADR:** [0001](0001-hexagonal-architecture.md)

## Контекст

API мають споживати:

- **Веб‑сторінка‑демо** (`/`): зручніше HTTP+JSON (Swagger UI, curl, Postman).
- **Можливі автоматизації / внутрішні сервіси:** надають перевагу строго типізованому транспорту (контракт у `.proto`, codegen під клієнти, бінарна серіалізація).

При цьому use‑case‑шар у `application/*` — один і той самий незалежно від каналу.

## Розглянуті варіанти

1. **Тільки HTTP (REST + JSON).** Найпростіше; пасує для веб‑демо й curl, але не дає типізованого SDK для іншого мовного стеку.
2. **Тільки gRPC.** Сильна типізація і кодогенерація, але незручно для браузера й Swagger‑огляду.
3. **HTTP + gRPC як два паралельні inbound‑адаптери** з спільними use‑case’ами.
4. **gRPC + grpc‑gateway** (генерувати REST з proto). Рятує від дубляжу контракту, але вводить ще одну залежність і кодогенерацію — для розміру цього сервісу надлишково.

## Рішення

Обрано **варіант 3**:

- **HTTP** на `:8080` через Gin (`internal/adapter/inbound/http`):
  - `POST /api/subscribe`, `GET /api/confirm/{token}`, `GET /api/unsubscribe/{token}`, `GET /api/subscriptions`;
  - middleware: `APIKeyAuth` (опційний), Prometheus метрики;
  - статика `/`, `/static/*`, OpenAPI на `/api/swagger.yaml`;
  - `/metrics` для Prometheus.
- **gRPC** на `:50051` (`internal/adapter/inbound/grpc`):
  - сервіс `subscription.SubscriptionService` із чотирма RPC, `pb` згенеровано через `buf.gen.yaml` із `api/proto/subscription.proto`;
  - server reflection увімкнено (`reflection.Register(srv)`), щоб `grpcurl` працював без клієнтського `.proto`.
- Обидва handler‑и приймають **той самий** `SubscriptionUseCase`; помилки доменного шару мапляться у відповідні HTTP‑коди / gRPC `status.Code`.

## Наслідки

**Плюси**

- Один use‑case, два канали — без дубляжу логіки.
- Веб‑клієнти й cURL/Postman продовжують працювати; внутрішні клієнти отримують `protoc`‑SDK.
- Запуск обох транспортів — паралельні goroutine у `main.go`, окремі порти й graceful shutdown.

**Мінуси / трейдофи**

- Контракт описано двічі: у `swagger.yaml` і у `.proto`. У майбутньому варто або генерувати REST з proto (grpc‑gateway), або визнати їх стабільними і покрити контрактними тестами.
- Дві серверні стеки → дві поверхні для CVE/налаштувань (TLS, лімітів, таймаутів).
- Reflection увімкнено постійно — зручно в dev, але в prod може бути вимкнено політикою безпеки.
