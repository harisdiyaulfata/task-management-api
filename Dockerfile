FROM golang:1.26-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/task-management-api ./cmd/api

FROM alpine:3.22 AS runtime

RUN addgroup -S app && adduser -S -G app app

WORKDIR /app
COPY --from=builder /out/task-management-api ./task-management-api
COPY --from=builder /src/db/migrations ./db/migrations

USER app

ENV APP_ENV=production \
    HTTP_PORT=8080 \
    MIGRATIONS_PATH=/app/db/migrations

EXPOSE 8080

ENTRYPOINT ["./task-management-api"]
