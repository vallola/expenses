package main

import (
	"context"
	stdhttp "net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/vallola/expenses/internal/config"
	"github.com/vallola/expenses/internal/logger"
	"github.com/vallola/expenses/internal/repository/postgres"
	"github.com/vallola/expenses/internal/service"
	http "github.com/vallola/expenses/internal/transport/http"
	"github.com/vallola/expenses/internal/transport/telegram"
	"go.uber.org/zap"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	cfg := config.New()
	log := logger.New()

	dbPool, err := postgres.NewPool(cfg.DbUrl, log)
	if err != nil {
		return
	}
	defer dbPool.Close()

	if err = postgres.Migrate(cfg.DbUrl, log); err != nil {
		return
	}
	log.Info("Database migrations applied successfully")

	repo := postgres.NewPostgres(dbPool)
	svc := service.NewExpenseService(repo)
	restServer := http.NewServer(svc, log)

	bot, err := telegram.NewBot(cfg.BotToken, svc, cfg, log)
	if err != nil {
		log.Error("Failed to initialize Telegram bot", zap.Error(err))
		return
	}

	httpServer := &stdhttp.Server{
		Addr:    cfg.ServerAddr,
		Handler: restServer,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	var wg sync.WaitGroup

	recovery(log, &wg, func() {
		log.Info("Admin web dashboard running", zap.String("addr", cfg.ServerAddr))
		if err := httpServer.ListenAndServe(); err != nil && err != stdhttp.ErrServerClosed {
			log.Error("HTTP server failed", zap.Error(err))
		}
	})

	recovery(log, &wg, func() {
		log.Info("Telegram bot running")
		bot.Start()
	})

	sig := <-quit
	log.Info("Shutting down application...", zap.String("signal", sig.String()))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Error("HTTP server forced to shutdown", zap.Error(err))
	} else {
		log.Info("HTTP server stopped gracefully")
	}

	bot.Stop()
	log.Info("Telegram bot stopped")

	wg.Wait()

	log.Info("Application stopped cleanly")
}

func recovery(log *zap.Logger, wg *sync.WaitGroup, fn func()) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Error("PANIC RECOVERED", zap.Any("panic", r), zap.String("stack", string(debug.Stack())))
			}
		}()
		fn()
	}()
}
