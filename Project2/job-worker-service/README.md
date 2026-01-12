# Job Worker Service (async background jobs)

Микросервис, который принимает “длительные задачи” через HTTP API, сохраняет их в PostgreSQL, кладёт в очередь Redis и обрабатывает в фоне воркером. Клиент может проверять статус и получать результат.

Проект демонстрирует:
- фоновые задачи / job worker pattern
- разделение ответственности API ↔ worker
- простую CQRS-форму: запись через `POST`, чтение через `GET`
- Redis как reliable queue (`BRPOPLPUSH` + processing list)
- приоритеты задач (high/normal/low)
- Docker Compose (app + worker + postgres + redis)
- Swagger/OpenAPI (swaggo)

---

## Архитектура

- **app**: HTTP API (создание задач, получение статуса/результата)
- **worker**: воркер-пул, который забирает задачи из Redis, выполняет и пишет результат в PostgreSQL
- **postgres**: хранит jobs
- **redis**: очередь задач (приоритетные lanes + processing map)

---

## Модель Job

Таблица `jobs` (PostgreSQL):

- `id` uuid
- `type` text
- `status` text: `pending | processing | done | error`
- `priority` int: `0 | 1 | 2`
- `input` jsonb
- `output` jsonb
- `error` text
- `created_at`, `updated_at`

---

## Priority

Поле `priority` влияет на порядок обработки задач:

- `2` — **high**
- `1` — **normal** (default)
- `0` — **low**

Если `priority` не указан или выходит за диапазон — используется `1`.

---

## Reliable queue + Priority lanes (Redis)

Чтобы задачи не терялись при падении воркера, используется паттерн “reliable queue” с двумя списками:
- **queue** — откуда воркер забирает задачи
- **processing** — куда задачи атомарно перемещаются при взятии воркером, и откуда удаляются при ACK

### Redis keys

Используются три приоритетные очереди и три processing-листа:

- `jobs:queue:high`   → `jobs:processing:high`
- `jobs:queue:normal` → `jobs:processing:normal`
- `jobs:queue:low`    → `jobs:processing:low`

Также используется hash-маппинг для корректного ACK:

- `jobs:processing:map`: `job_id -> processing_list_key`

---

## Swagger / OpenAPI

Swagger UI доступен по адресу:

- `http://localhost:8080/swagger/index.html`

### Как обновить документацию

После изменения DTO/handlers нужно перегенерировать `docs/`:

```powershell
swag init -g cmd/app/main.go -o docs --parseDependency --parseInternal


### Поток обработки

1) **App** создаёт job в PostgreSQL со статусом `pending`
2) **App** кладёт `job_id` в Redis lane в зависимости от `priority`:
    - `LPUSH jobs:queue:{high|normal|low} job_id`
3) **Worker** атомарно забирает задачу:
    - `BRPOPLPUSH jobs:queue:high jobs:processing:high`
    - если пусто — пробует `normal`, затем `low`
4) Worker фиксирует маппинг:
    - `HSET jobs:processing:map job_id jobs:processing:<lane>`
5) Worker обновляет статус в Postgres (`processing`), выполняет работу, пишет `output` или `error`
6) Worker подтверждает обработку (ACK):
    - `LREM jobs:processing:<lane> 1 job_id`
    - `HDEL jobs:processing:map job_id`

### Reaper (возврат “зависших” задач)

Если воркер упал, `job_id` может остаться в `jobs:processing:*`.
Reaper периодически возвращает часть элементов обратно в queue:

- `RPOPLPUSH jobs:processing:<lane> jobs:queue:<lane>`

Это даёт гарантию **at-least-once** (задача может выполниться повторно — в production решают идемпотентностью/attempts/DLQ).
```
---

## Запуск

### Требования
- Docker + Docker Compose

### Старт
```bash
docker compose up --build
