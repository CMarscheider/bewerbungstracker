// Package testdb startet eine echte Postgres-Instanz für Integrationstests.
package testdb

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"bewerbungsmanager/internal/db"
)

// Start startet einen Postgres-Container, migriert ihn und liefert Pool, URL und Aufräumfunktion.
func Start(ctx context.Context) (*pgxpool.Pool, string, func(), error) {
	ctr, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		_ = testcontainers.TerminateContainer(ctr)
		return nil, "", nil, fmt.Errorf("container starten: %w", err)
	}
	terminate := func() { _ = testcontainers.TerminateContainer(ctr) }

	url, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		terminate()
		return nil, "", nil, err
	}
	if err := db.Migrate(ctx, url); err != nil {
		terminate()
		return nil, "", nil, err
	}
	pool, err := db.Connect(ctx, url)
	if err != nil {
		terminate()
		return nil, "", nil, err
	}
	return pool, url, func() { pool.Close(); terminate() }, nil
}

// Reset leert alle Tabellen; zu Beginn jedes Tests aufrufen.
func Reset(t testing.TB, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), "TRUNCATE companies, applications, application_events CASCADE")
	if err != nil {
		t.Fatalf("Tabellen leeren: %v", err)
	}
}
