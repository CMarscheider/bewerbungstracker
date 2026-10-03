// Command server startet die Bewerbungs-Tracker-API.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // Zeitzonen auch im minimalen Container-Image

	"bewerbungsmanager/internal/config"
	"bewerbungsmanager/internal/db"
	"bewerbungsmanager/internal/documents"
	"bewerbungsmanager/internal/httpapi"
	"bewerbungsmanager/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("server beendet", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := db.Migrate(ctx, cfg.DatabaseURL, logger); err != nil {
		return err
	}
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	var opts []service.Option
	if cfg.GotenbergURL != "" {
		opts = append(opts, service.WithPDFConverter(documents.NewGotenberg(cfg.GotenbergURL)))
	} else {
		logger.Warn("GOTENBERG_URL nicht gesetzt – PDF-Erzeugung ist deaktiviert")
	}
	handler, err := httpapi.NewRouter(service.New(pool, time.Now, opts...), logger)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server läuft", "port", cfg.Port)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}
	stop() // zweites Signal beendet den Prozess sofort

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	logger.Info("server fährt herunter")
	return srv.Shutdown(shutdownCtx)
}
