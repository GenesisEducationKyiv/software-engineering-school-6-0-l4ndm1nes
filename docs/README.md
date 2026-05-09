# Документація

Каталог містить інженерну документацію проєкту **GitHub Release Notification API**.

- [`system-design.md`](system-design.md) — System Design: контекст, контейнери, модель даних, основні потоки, нефункціональні вимоги, ризики й еволюція.
- [`adr/`](adr/) — Architectural Decision Records у форматі MADR. Кожен файл — одне рішення з мотивацією та наслідками.

## ADR

| #   | Назва                                                                  | Статус   |
| --- | ---------------------------------------------------------------------- | -------- |
| 001 | [Hexagonal architecture (ports & adapters)](adr/0001-hexagonal-architecture.md) | Прийнято |
| 002 | [Polling GitHub API замість webhooks](adr/0002-polling-vs-webhooks.md) | Прийнято |
| 003 | [PostgreSQL як основне сховище](adr/0003-postgres-as-primary-store.md) | Прийнято |
| 004 | [Redis як кеш для GitHub API](adr/0004-redis-cache-for-github.md)      | Прийнято |
| 005 | [Email через SMTP (MailHog у dev, AWS SES у prod)](adr/0005-smtp-email-delivery.md) | Прийнято |
| 006 | [HTTP (REST) і gRPC як два транспортних адаптери](adr/0006-http-and-grpc-transports.md) | Прийнято |
| 007 | [Confirmation/unsubscribe через одноразові токени](adr/0007-token-based-confirmation.md) | Прийнято |

## Як підтримувати

- Кожне нове архітектурно значуще рішення → новий ADR (`docs/adr/NNNN-short-title.md`).
- Якщо змінюється контейнер/потік даних — оновити `system-design.md` (зокрема Mermaid-діаграми).
- ADR не редагуємо «заднім числом»: якщо рішення скасовано — створюємо новий ADR зі статусом «Замінює ADR-XXXX», у старому проставляємо «Замінено ADR-YYYY».
