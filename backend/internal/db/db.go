// Package db verbindet sich mit Postgres und führt Migrationen aus.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registriert den Treiber "pgx" für goose
	"github.com/pressly/goose/v3"

	"bewerbungsmanager/migrations"
)

// Connect öffnet einen Verbindungspool und prüft die Erreichbarkeit.
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("pool anlegen: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("datenbank nicht erreichbar: %w", err)
	}
	return pool, nil
}

// Migrate bringt das Schema auf den neuesten Stand.
func Migrate(ctx context.Context, url string, logger *slog.Logger) error {
	sqlDB, err := sql.Open("pgx", url)
	if err != nil {
		return fmt.Errorf("datenbank öffnen: %w", err)
	}
	defer sqlDB.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		return fmt.Errorf("goose: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("migrationen: %w", err)
	}
	versions := make([]int64, 0, len(results))
	for _, r := range results {
		versions = append(versions, r.Source.Version)
	}
	logger.Info("migrationen angewendet", "anzahl", len(results), "versionen", versions)
	return nil
}
