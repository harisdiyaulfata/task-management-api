package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/example/task-management-api/configs"
	delivery "github.com/example/task-management-api/internal/delivery/http"
	"github.com/example/task-management-api/internal/delivery/http/handler"
	"github.com/example/task-management-api/internal/platform/cache"
	"github.com/example/task-management-api/internal/platform/database"
	"github.com/example/task-management-api/internal/platform/jwt"
	"github.com/example/task-management-api/internal/platform/logger"
	"github.com/example/task-management-api/internal/platform/notification"
	postgresrepo "github.com/example/task-management-api/internal/repository/postgres"
	redisrepo "github.com/example/task-management-api/internal/repository/redis"
	"github.com/example/task-management-api/internal/usecase"
)

func main() {
	log := logger.NewJSON(os.Stdout)
	cfg, err := configs.Load()
	if err != nil {
		log.Error("load configuration", "error", err)
		os.Exit(1)
	}

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStartup()

	postgresPool, err := database.NewPostgresPool(startupCtx, cfg.Postgres)
	if err != nil {
		log.Error("connect PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer postgresPool.Close()
	if err := database.RunMigrations(cfg.Postgres.DSN, cfg.MigrationsPath); err != nil {
		log.Error("run database migrations", "error", err)
		os.Exit(1)
	}
	log.Info("database migrations are up to date", "path", cfg.MigrationsPath)

	redisClient, err := cache.NewRedisClient(startupCtx, cfg.Redis)
	if err != nil {
		log.Error("connect Redis", "error", err)
		os.Exit(1)
	}
	defer func() { _ = redisClient.Close() }()

	tokens, err := jwt.NewManager(cfg.JWT.Secret, cfg.JWT.TTL)
	if err != nil {
		log.Error("initialize JWT manager", "error", err)
		os.Exit(1)
	}

	users := postgresrepo.NewUserRepository(postgresPool)
	tasks := postgresrepo.NewTaskRepository(postgresPool)
	teams := postgresrepo.NewTeamRepository(postgresPool)
	sessions := redisrepo.NewSessionRepository(redisClient)
	idempotency := redisrepo.NewIdempotencyRepository(redisClient)
	notifier := notification.NewLogNotifier(log)

	authUsecase := usecase.NewAuthUsecase(users, sessions, tokens)
	taskUsecase := usecase.NewTaskUsecaseWithDependencies(tasks, idempotency, notifier)
	teamUsecase := usecase.NewTeamUsecase(teams)
	router := delivery.NewRouter(
		log,
		handler.NewAuthHandler(authUsecase),
		handler.NewTaskHandler(taskUsecase),
		handler.NewTeamHandler(teamUsecase),
		tokens,
		sessions,
	)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.HTTPPort),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Info("HTTP server starting", "port", cfg.HTTPPort, "environment", cfg.AppEnv)
		serverErrors <- server.ListenAndServe()
	}()

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	case signal := <-shutdownSignal:
		log.Info("shutdown signal received", "signal", signal.String())
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "error", err)
		return
	}
	log.Info("HTTP server stopped")
}
