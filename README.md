# Async Import Service (Go)

Простой pet-проект про асинхронный backend на Go.
Идея: API быстро принимает импорт, кладет задачу в очередь, а отдельный worker делает работу в фоне.

## Зачем этот проект

Я сделал его, чтобы показать на резюме и собеседовании:

- Go backend разработку
- работу с очередями (RabbitMQ)
- разделение API и фоновой обработки
- хранение статусов и результатов в PostgreSQL
- запуск всего проекта через Docker Compose

## Как он работает

1. Клиент отправляет `POST /imports` с JSON.
2. API создает задачу в таблице `tasks` со статусом `queued`.
3. API отправляет в RabbitMQ сообщение с `task_id`.
4. Worker забирает сообщение из очереди.
5. Worker ставит статус `processing`, валидирует данные, сохраняет клиентов в БД.
6. После обработки worker записывает результат (сколько успешно, сколько с ошибкой) и ставит `done`.
7. Если случилась критическая ошибка, статус становится `failed`.

## Что внутри

- `api` — HTTP сервис
- `worker` — фоновый обработчик задач
- `postgres` — хранение задач и клиентов
- `rabbitmq` — очередь между API и worker

## Технологии

- Go 1.25+
- chi
- pgx
- amqp091-go
- PostgreSQL
- RabbitMQ
- Docker Compose
- slog

## Запуск

Создать .env файл по аналогии как .env.example
```bash
docker compose up --build
```

После старта:

- API: `http://localhost:8080`
- RabbitMQ UI: `http://localhost:15672` (логин/пароль `guest/guest`)

## API

### Создать импорт

```bash
POST http://localhost:8080/imports
    "clients": [
      {"name":"Ivan", "email":"ivan@example.com", "phone":"+79990000000"},
      {"name":"Petr", "email":"bad-email", "phone":"+79990000001"},
      {"name":"Ivan Duplicate", "email":"ivan@example.com"}
    ]
```

Пример ответа:

```json
{
  "task_id": "a4ea7bbf-91fd-4aa8-b64b-dd85f2453ec5",
  "status": "queued"
}
```

## Правила обработки данных

- `name` обязателен
- `email` обязателен и валидируется
- `phone` не обязателен
- дубликаты по `email` не добавляются второй раз
- ошибки по строкам сохраняются в результате задачи

## Статусы задач

- `queued`
- `processing`
- `done`
- `failed`

## Retry + DLQ

В проекте добавлен механизм повторных попыток:

- при ошибке обработки сообщение уходит в `RABBITMQ_RETRY_QUEUE`
- retry очередь держит сообщение `RABBITMQ_RETRY_DELAY_MS` и возвращает его обратно в основную очередь
- количество попыток ограничено `RABBITMQ_MAX_RETRIES`
- после превышения лимита сообщение уходит в `RABBITMQ_DLQ_QUEUE`

## Таблицы в БД

### `tasks`

- id
- type
- status
- payload (JSONB)
- result (JSONB)
- error_message
- created_at
- updated_at

### `clients`

- id
- name
- email (уникальный)
- phone
- created_at

## Тесты

Что покрыто:

- валидация email/payload
- сервис создания задачи
- happy-path тест worker-а