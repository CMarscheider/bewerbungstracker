package db_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"bewerbungsmanager/internal/db"
	"bewerbungsmanager/internal/testdb"
)

func TestMigrateCreatesSchemaAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool, url, cleanup, err := testdb.Start(ctx)
	if err != nil {
		t.Fatalf("Testdatenbank starten: %v", err)
	}
	defer cleanup()

	// testdb.Start hat bereits migriert; ein zweiter Lauf darf nichts kaputt machen.
	if err := db.Migrate(ctx, url, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("zweite Migration: %v", err)
	}

	var n int
	err = pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.tables
		WHERE table_schema = 'public'
		  AND table_name IN ('companies', 'applications', 'application_events', 'latest_events')`).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Errorf("erwartet 4 Tabellen/Views, gefunden %d", n)
	}
}
