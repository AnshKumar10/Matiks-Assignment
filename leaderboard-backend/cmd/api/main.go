package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AnshKumar10/Matiks-Assignment/internal/config"
	apphttp "github.com/AnshKumar10/Matiks-Assignment/internal/http"
	"github.com/AnshKumar10/Matiks-Assignment/internal/leaderboard"
	"github.com/AnshKumar10/Matiks-Assignment/internal/logger"
	"github.com/AnshKumar10/Matiks-Assignment/internal/storage/postgres"
	"github.com/AnshKumar10/Matiks-Assignment/internal/storage/redis"
	"go.uber.org/zap"
)

func main() {
	cfg := config.Load()
	log := logger.New(cfg.AppEnv)
	defer log.Sync()

	ctx := context.Background()

	pg, err := postgres.New(cfg.PostgresDSN)
	if err != nil {
		log.Fatal("postgres connection failed", zap.Error(err))
	}

	redisClient := redis.New(cfg.RedisAddr, cfg.RedisPassword)
	if err := redisClient.Ping(ctx); err != nil {
		log.Fatal("redis connection failed", zap.Error(err))
	}

	lbRepo := leaderboard.NewRepository(pg, redisClient.RDB)
	lbService := leaderboard.NewService(lbRepo)

	router := apphttp.NewRouter(lbService)

	server := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: router,
	}

	go func() {
		log.Info("HTTP server started", zap.String("port", cfg.HTTPPort))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	<-shutdown

	log.Info("shutting down server")

	ctxShutdown, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	server.Shutdown(ctxShutdown)
}
