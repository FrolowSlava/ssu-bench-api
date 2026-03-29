# SsuBench API

REST API платформа для управления задачами между заказчиками и исполнителями с системой безопасных платежей виртуальными баллами.

## Стек Технологий

| Компонент | Технология | Версия |
|-----------|------------|--------|
| **Язык** | Go (Golang) | 1.21+ |
| **Фреймворк** | Gin | latest |
| **База данных** | PostgreSQL | 15+ |
| **Драйвер БД** | pgx | v5 |
| **Аутентификация** | JWT (HS256) | - |
| **Хеширование** | bcrypt | cost=10 |
| **Миграции** | golang-migrate | latest |
| **Контейнеризация** | Docker + Docker Compose | 20.x+ |
| **Документация** | OpenAPI 3.0 + Swagger UI | - |

## О Проекте

**SsuBench** — это платформа, где заказчики (`customer`) размещают задачи с бюджетом, исполнители (`executor`) создают отклики и выполняют работу, а администраторы (`admin`) управляют пользователями и отслеживают платежи. Система обеспечивает **атомарный перевод баллов** при подтверждении выполнения задачи.

### Ключевые Возможности

-  **Ролевая модель**: 3 роли (customer, executor, admin) с RBAC
-  **Безопасная аутентификация**: JWT-токены + bcrypt
-  **Атомарные платежи**: транзакции с `FOR UPDATE` + Serializable isolation
-  **Статусные переходы**: задачи проходят жизненный цикл (open → in_progress → completed/cancelled)
-  **Админ-панель**: управление пользователями, блокировка, просмотр платежей
-  **Пагинация и фильтрация**: для всех списков
-  **OpenAPI 3.0**: полная документация + интерактивный Swagger UI
-  **13 интеграционных тестов**: покрывают всю бизнес-логику

### Основной Workflow

1. Customer создаёт задачу (статус: open) — POST /api/v1/tasks
2. Executor создаёт отклик — POST /api/v1/tasks/{id}/bids
3. Customer выбирает исполнителя (статус: in_progress) — POST /api/v1/tasks/{id}/select-bid
4. Executor завершает работу — POST /api/v1/bids/{id}/complete
5. Customer подтверждает выполнение (статус: completed) — POST /api/v1/tasks/{id}/confirm
   → Атомарно: списываются баллы у заказчика, начисляются исполнителю

## Быстрый Старт

### 1. Настройте окружение
`.env.example` содержит рабочие значения для локальной разработки (менять ничего не нужно).
```bash
cp .env.example .env
```

### 2. Запустите базу данных

```bash
docker-compose up -d
```
Миграции применяются автоматически (папка migrations).

### 3. Установите зависимости

```bash
go mod tidy
```

### 4. Запустите сервер

```bash
go run cmd/main.go
```

### 5. Проверьте работоспособность

```bash
curl http://localhost:8080/health
```

### 6. Откройте Swagger UI

```bash
docker-compose -f docker-compose.swagger.yaml up -d
```
После этой команды swagger доступен на http://localhost:8081/

## API Endpoints

### Health
- `GET /health` — Проверка состояния сервера

### Auth
- `POST /api/v1/auth/register` — Регистрация
- `POST /api/v1/auth/login` — Логин
- `GET /api/v1/me` — Профиль

### Tasks
- `GET /api/v1/tasks` — Список задач
- `POST /api/v1/tasks` — Создать задачу
- `GET /api/v1/tasks/:id` — Задача по ID
- `POST /api/v1/tasks/:id/select-bid` — Выбрать отклик
- `POST /api/v1/tasks/:id/confirm` — Подтвердить выполнение
- `POST /api/v1/tasks/:id/cancel` — Отменить задачу

### Bids
- `POST /api/v1/tasks/:id/bids` — Создать отклик
- `POST /api/v1/bids/:id/complete` — Завершить отклик

### Admin
- `GET /api/v1/admin/users` — Список пользователей
- `GET /api/v1/admin/users/:id` — Информация о пользователе
- `POST /api/v1/admin/users/:id/block` — Блокировка
- `POST /api/v1/admin/users/:id/unblock` — Разблокировка
- `GET /api/v1/admin/payments` — Список платежей
- `GET /api/v1/admin/tasks` — Все задачи


## Переменные окружения

- `GIN_MODE` — режим работы Gin (debug/release)
- `PORT` — порт HTTP-сервера
- `HTTP_READ_TIMEOUT, HTTP_WRITE_TIMEOUT, HTTP_IDLE_TIMEOUT` — таймауты HTTP-сервера
- `DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE` — подключение к PostgreSQL
- `JWT_SECRET` — секретный ключ для подписи JWT (мин. 32 байта)
- `JWT_EXPIRES_IN` — время жизни JWT-токена


## Примеры curl

### Health Check

```bash
# GET /health — Проверка состояния сервера
Invoke-RestMethod http://localhost:8080/health
```

### Регистрация пользователей

```bash
# POST /api/v1/auth/register — Customer с балансом 100 для тестов
$body = @{username="customer";email="customer@test.com";password="Pass123!";role="customer";balance=100} | ConvertTo-Json
Invoke-RestMethod -Uri http://localhost:8080/api/v1/auth/register -Method Post -ContentType "application/json" -Body $body

# POST /api/v1/auth/register — Executor (баланс 0 по умолчанию)
$body = @{username="executor";email="executor@test.com";password="Pass123!";role="executor"} | ConvertTo-Json
Invoke-RestMethod -Uri http://localhost:8080/api/v1/auth/register -Method Post -ContentType "application/json" -Body $body

# POST /api/v1/auth/register — Admin
$body = @{username="admin";email="admin@test.com";password="Pass123!";role="admin"} | ConvertTo-Json
Invoke-RestMethod -Uri http://localhost:8080/api/v1/auth/register -Method Post -ContentType "application/json" -Body $body
```

### Логин и получение токенов

```bash
# POST /api/v1/auth/login — Customer
$body = @{email="customer@test.com";password="Pass123!"} | ConvertTo-Json
$customerToken = (Invoke-RestMethod -Uri http://localhost:8080/api/v1/auth/login -Method Post -ContentType "application/json" -Body $body).token

# POST /api/v1/auth/login — Executor
$body = @{email="executor@test.com";password="Pass123!"} | ConvertTo-Json
$executorToken = (Invoke-RestMethod -Uri http://localhost:8080/api/v1/auth/login -Method Post -ContentType "application/json" -Body $body).token

# POST /api/v1/auth/login — Admin
$body = @{email="admin@test.com";password="Pass123!"} | ConvertTo-Json
$adminToken = (Invoke-RestMethod -Uri http://localhost:8080/api/v1/auth/login -Method Post -ContentType "application/json" -Body $body).token
```


###  Профиль текущего пользователя

```bash
# GET /api/v1/me
Invoke-RestMethod -Uri http://localhost:8080/api/v1/me -Headers @{Authorization="Bearer $customerToken"}
```


### Задачи

```bash
# GET /api/v1/tasks — Список задач
$tasks = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/tasks?page=1&limit=20" -Headers @{Authorization="Bearer $customerToken"}

# POST /api/v1/tasks — Создать задачу (Customer)
$body = @{title="Dev service";description="need webpage";budget=100} | ConvertTo-Json
$task = Invoke-RestMethod -Uri http://localhost:8080/api/v1/tasks -Method Post -ContentType "application/json" -Headers @{Authorization="Bearer $customerToken"} -Body $body
$taskId = $task.id

# GET /api/v1/tasks/:id — Задача по ID
$taskById = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/tasks/$taskId" -Headers @{Authorization="Bearer $customerToken"}

# POST /api/v1/tasks/:id/cancel — Отменить задачу (опционально)
# $cancel = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/tasks/$taskId/cancel" -Method Post -Headers @{Authorization="Bearer $customerToken"}
```


### Отклики

```bash
# POST /api/v1/tasks/:id/bids — Создать отклик (Executor)
$body = @{amount=80} | ConvertTo-Json
$bid = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/tasks/$taskId/bids" -Method Post -ContentType "application/json" -Headers @{Authorization="Bearer $executorToken"} -Body $body
$bidId = $bid.id

# POST /api/v1/tasks/:id/select-bid — Выбрать исполнителя (Customer)
# ВАЖНО: Должно быть ПЕРЕД завершением отклика!
$body = @{bid_id=$bidId} | ConvertTo-Json
Invoke-RestMethod -Uri "http://localhost:8080/api/v1/tasks/$taskId/select-bid" -Method Post -ContentType "application/json" -Headers @{Authorization="Bearer $customerToken"} -Body $body

# POST /api/v1/bids/:id/complete — Завершить отклик (Executor)
# Только после select-bid!
Invoke-RestMethod -Uri "http://localhost:8080/api/v1/bids/$bidId/complete" -Method Post -Headers @{Authorization="Bearer $executorToken"}
```


### Подтверждение + Оплата

```bash
# POST /api/v1/tasks/:id/confirm — Подтвердить выполнение + атомарная оплата (Customer)
# Только после complete! Списывает баланс у заказчика, начисляет исполнителю
$confirm = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/tasks/$taskId/confirm" -Method Post -Headers @{Authorization="Bearer $customerToken"}

# Проверка итоговых балансов
$customerMe = Invoke-RestMethod -Uri http://localhost:8080/api/v1/me -Headers @{Authorization="Bearer $customerToken"}
$executorMe = Invoke-RestMethod -Uri http://localhost:8080/api/v1/me -Headers @{Authorization="Bearer $executorToken"}
Write-Host "Customer: $($customerMe.balance) | Executor: $($executorMe.balance)"
```


### Админские операции

```bash
# GET /api/v1/admin/users — Список пользователей
$users = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/admin/users?page=1&limit=20" -Headers @{Authorization="Bearer $adminToken"}
Write-Host "Пользователей: $($users.pagination.total)"


# GET /api/v1/admin/users/:id — Информация о пользователе
$userId = 1
$userDetails = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/admin/users/$userId" -Headers @{Authorization="Bearer $adminToken"}
Write-Host "Пользователь #$($userDetails.id): $($userDetails.username)"


# POST /api/v1/admin/users/:id/block — Заблокировать пользователя
# Админа нельзя заблокировать — проверяем роль
if ($userDetails.role -ne "admin") {
    $block = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/admin/users/$userId/block" -Method Post -Headers @{Authorization="Bearer $adminToken"}
    Write-Host "$($block.message)"
} else {
    Write-Host "Пропускаем блокировку админа"
}


# POST /api/v1/admin/users/:id/unblock — Разблокировать пользователя
if ($userDetails.role -ne "admin") {
    $unblock = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/admin/users/$userId/unblock" -Method Post -Headers @{Authorization="Bearer $adminToken"}
    Write-Host "$($unblock.message)"
}


# GET /api/v1/admin/payments — Список платежей
$payments = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/admin/payments?page=1&limit=20" -Headers @{Authorization="Bearer $adminToken"}
Write-Host "Платежей: $($payments.pagination.total)"
if ($payments.payments) {
    $payments.payments | ForEach-Object {
        Write-Host "  - $($_.amount) баллов: $($_.from_user_id) → $($_.to_user_id) ($($_.type))"
    }
}


# GET /api/v1/admin/tasks — Список всех задач
$allTasks = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/admin/tasks?page=1&limit=20" -Headers @{Authorization="Bearer $adminToken"}
Write-Host "Задач: $($allTasks.pagination.total)"
if ($allTasks.tasks) {
    $allTasks.tasks | ForEach-Object {
        Write-Host "  - #$($_.id): $($_.title) [статус: $($_.status), бюджет: $($_.budget)]"
    }
}
```


### Тесты
Создание тестовой БД
```
docker exec -it ssubench-postgres psql -U postgres -c "CREATE DATABASE ssubench_test;" 2>$null
```
Миграции к тестовой БД
```
Get-Content migrations/001_create_users_table.sql | docker exec -i ssubench-postgres psql -U postgres -d ssubench_test
Get-Content migrations/002_create_tasks_table.sql | docker exec -i ssubench-postgres psql -U postgres -d ssubench_test
Get-Content migrations/003_create_bids_table.sql | docker exec -i ssubench-postgres psql -U postgres -d ssubench_test
Get-Content migrations/004_create_payments_table.sql | docker exec -i ssubench-postgres psql -U postgres -d ssubench_test
```
Запуск тестов
```
go test ./internal/service -v
```