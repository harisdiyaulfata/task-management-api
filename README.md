# Task Management API

A production-oriented REST API for multi-user task and team management. It supports JWT authentication, Redis-backed sessions, owner-scoped task CRUD, idempotent task creation, team membership management, and transactional task assignment.

## Tech stack

- Go 1.26
- Gin HTTP framework
- PostgreSQL 17
- Redis 7
- JWT (`golang-jwt/jwt`)
- `pgx` PostgreSQL driver and connection pool
- `golang-migrate` for schema migrations
- Docker and Docker Compose

## Architecture

The project follows Clean Architecture. Dependencies point inward, keeping business rules separate from transport and infrastructure concerns.

```text
Delivery (Gin handlers and middleware)
        v
Usecase (application rules)
        v
Domain (entities, interfaces, business contracts)
        ^
Repository / Platform (PostgreSQL, Redis, JWT, logging, notifications)
```

- **Domain** — Defines entities (`User`, `Task`), repository contracts, status values, and domain errors. It has no knowledge of Gin, PostgreSQL, or Redis.
- **Repository** — Implements domain contracts using PostgreSQL for persistent data and Redis for sessions/idempotency records.
- **Usecase** — Contains authentication and task workflows, validation, pagination defaults, idempotent creation, and assignment orchestration.
- **Delivery** — Gin routes, request/response mapping, JWT middleware, error handling, request IDs, and structured request logs.
- **Platform** — Reusable technical integrations: connection pools, JWT signing, JSON logging, migration runner, and mock notifications.

### Identifier strategy

PostgreSQL uses `BIGINT id` primary keys and `BIGINT` foreign keys internally for compact indexes and efficient joins. `users` and `tasks` also have a unique UUID `public_id`. Repository queries translate between the two, so the API, JWT claims, and URLs continue to expose UUIDs only; internal sequential identifiers are never returned to clients.

## Run with Docker

Prerequisites: Docker Engine and Docker Compose v2.

1. Build and start every service:

   ```bash
   docker compose up --build
   ```

2. Wait for the API startup log stating that migrations are up to date.

3. Confirm the service is running:

   ```bash
   curl http://localhost:8080/health
   ```

   Expected response:

   ```json
   {"status":"ok"}
   ```

The Compose stack exposes:

| Service | Host port |
| --- | --- |
| API | `8080` |
| PostgreSQL | `5432` |
| Redis | `6379` |

PostgreSQL data persists in the named Docker volume `postgres_data`. The Compose credentials are intended for local development only; replace `JWT_SECRET` and database credentials before deploying elsewhere.

To stop containers while retaining database data:

```bash
docker compose down
```

To remove containers and database data:

```bash
docker compose down -v
```

## Run without Docker

Prerequisites: Go 1.26+, PostgreSQL, and Redis.

1. Create a PostgreSQL database and ensure Redis is running.

2. The repository includes a local `.env` file with development defaults. Update its PostgreSQL credentials and `JWT_SECRET` if your local services differ. The application loads this file automatically.

   Alternatively, set environment variables directly. Existing environment variables override values from `.env`:

   ```powershell
   $env:POSTGRES_DSN = "postgres://task_user:task_password@localhost:5432/task_management?sslmode=disable"
   $env:REDIS_ADDR = "localhost:6379"
   $env:REDIS_PASSWORD = ""
   $env:REDIS_DB = "0"
   $env:JWT_SECRET = "replace-with-a-long-random-secret"
   $env:JWT_TTL = "24h"
   $env:HTTP_PORT = "8080"
   $env:MIGRATIONS_PATH = "db/migrations"
   ```

3. Download dependencies and start the API:

   ```bash
   go mod download
   go run ./cmd/api
   ```

Database migrations in `db/migrations/*.up.sql` run automatically during startup. The database account needs permission to create the `pgcrypto` extension on the first run.

## Test

All unit tests use mocks; they do not require PostgreSQL or Redis.

```bash
go test ./...
```

The idempotency test starts 50 concurrent goroutines with the same `Idempotency-Key`, asserts that exactly one task is persisted, and verifies that all callers receive the same stored response:

```bash
go test ./internal/usecase -run TestCreateIdempotent_ConcurrentRequestsCreateExactlyOneTask -count=50
```

The sequential test verifies that a second request with the same key replays the original `201` status/body without creating another task:

```bash
go test ./internal/usecase -run TestCreateIdempotent_SequentialDuplicateReplaysOriginalResponse
```

On a machine with CGO enabled, also execute Go's race detector:

```bash
go test -race ./internal/usecase -run TestCreateIdempotent_ConcurrentRequestsCreateExactlyOneTask
```

## API endpoints

All task and team endpoints, plus logout, require `Authorization: Bearer <access-token>`.

| Method | Endpoint | Description |
| --- | --- | --- |
| `POST` | `/auth/register` | Register a user. |
| `POST` | `/auth/login` | Authenticate and receive a JWT access token. |
| `POST` | `/auth/logout` | Invalidate the current Redis-backed session. |
| `POST` | `/teams` | Create a team and add the authenticated user as a member. |
| `GET` | `/teams` | List teams for the authenticated user. |
| `GET` | `/teams/:id/members` | List members of a team the authenticated user belongs to. |
| `POST` | `/teams/:id/members` | Add a registered user to a team the authenticated user belongs to. |
| `DELETE` | `/teams/:id/members/:userId` | Remove a member from a team the authenticated user belongs to. |
| `POST` | `/tasks` | Create a task. Requires an `Idempotency-Key` UUID header. |
| `GET` | `/tasks` | List the authenticated user's tasks. Supports `page`, `limit`, `search`, and `status`. |
| `GET` | `/tasks/:id` | Get one owned task. |
| `PUT` | `/tasks/:id` | Update one owned task. |
| `DELETE` | `/tasks/:id` | Delete one owned task. |
| `POST` | `/tasks/:id/assign` | Assign an owned task to another user. |
| `GET` | `/health` | Liveness endpoint. |

## Team management

Team membership controls task assignment. The migration creates a `Default Team` and enrolls existing and newly registered users, so assignment is usable immediately.

- Creating a team automatically adds its creator as a member.
- Every member of a team may list its members, add a registered user, or remove another member (or themselves).
- Adding an existing member is idempotent and returns success without duplicating membership.
- Member-management endpoints return `403 FORBIDDEN` when the requester is not a member of the target team.
- `POST /tasks/:id/assign` requires the task owner and assignee to share at least one team. Assignment across different teams returns `403 FORBIDDEN`.

Example: add an existing user to a team:

```bash
curl -X POST http://localhost:8080/teams/<team-uuid>/members \
  -H "Authorization: Bearer <access-token>" \
  -H "Content-Type: application/json" \
  -d '{"user_id":"<user-public-uuid>"}'
```

### Example: create a task

```bash
curl -X POST http://localhost:8080/tasks \
  -H "Authorization: Bearer <access-token>" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: 2b7dbd24-4d56-4b6f-8cd3-2d7d2ea67cc0" \
  -d '{"title":"Prepare technical submission","status":"todo"}'
```

## Key features

### Idempotent task creation

`POST /tasks` requires an `Idempotency-Key` UUID. Redis atomically claims a key using `SET NX` and stores it for 24 hours. The request that owns the key creates the task, serializes the `201` response, and saves that exact status/body in Redis. Concurrent or retried requests with the same key wait for completion and replay the original response instead of inserting another task. The key is scoped to the authenticated user.

### Transactional assignment integrity

`POST /tasks/:id/assign` executes its workflow in one PostgreSQL transaction:

1. Confirm the assignee exists and lock the owned task.
2. Confirm owner and assignee share at least one team.
3. Update the task's `assignee_id`.
4. Insert an audit record into `task_logs` with previous and new assignee data.
5. Trigger the current mock notification implementation.
6. Commit only if every step succeeds; otherwise rollback the database changes.

The migration creates a `Default Team` and automatically adds registered users to it. Additional teams and memberships can be managed through the Team Management endpoints.

## Postman testing

Import the following files into Postman, then select the local environment:

- `postman/task-management-api.postman_collection.json`
- `postman/task-management-api.local.postman_environment.json`

Run the collection from its root in Collection Runner. It performs the complete scenario in order: authentication, task CRUD, sequential idempotency replay, successful assignment in the same team, failed assignment across different teams, team member add/remove, logout, and standard error responses. The collection generates a new idempotency key and unique team name for every run.

## Observability and errors

Each request gets an `X-Request-ID` UUID and emits a structured JSON log containing request ID, HTTP method, path, status, latency, and a level (`INFO` for 2xx/3xx, `WARN` for 4xx, `ERROR` for 5xx).

Errors use a consistent JSON structure:

```json
{
  "status": 400,
  "code": "INVALID_INPUT",
  "message": "request contains invalid data",
  "timestamp": "2026-09-01T00:00:00Z"
}
```
