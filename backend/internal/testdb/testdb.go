// Package testdb startet eine echte Postgres-Instanz für Integrationstests.
package testdb

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"bewerbungsmanager/internal/db"
)

// windowsDockerHost ist die Named Pipe, die Docker Desktop unter Windows bereitstellt.
const windowsDockerHost = "npipe:////./pipe/docker_engine"

// EnsureDockerHost setzt unter Windows DOCKER_HOST auf die Named Pipe von Docker Desktop.
// testcontainers erkennt Docker unter Windows per os.Stat auf die Named Pipe. Nutzen mehrere
// Testpakete parallel Docker, scheitert das mit "All pipe instances are busy" und endet in der
// irreführenden Meldung "rootless Docker is not supported on Windows". Mit gesetztem DOCKER_HOST
// entfällt diese Prüfung; der Docker-Client wartet bei belegter Pipe, statt abzubrechen.
func EnsureDockerHost() error {
	if runtime.GOOS == "windows" && os.Getenv("DOCKER_HOST") == "" {
		if err := os.Setenv("DOCKER_HOST", windowsDockerHost); err != nil {
			return fmt.Errorf("DOCKER_HOST setzen: %w", err)
		}
	}
	return nil
}

// Start startet einen Postgres-Container, migriert ihn und liefert Pool, URL und Aufräumfunktion.
func Start(ctx context.Context) (*pgxpool.Pool, string, func(), error) {
	if err := EnsureDockerHost(); err != nil {
		return nil, "", nil, err
	}

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
	if err := db.Migrate(ctx, url, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
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
	_, err := pool.Exec(context.Background(), "TRUNCATE companies, applications, application_events, application_documents, cv, cv_photo, cv_reviews CASCADE")
	if err != nil {
		t.Fatalf("Tabellen leeren: %v", err)
	}
}
