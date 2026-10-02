package httpapi_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"bewerbungsmanager/internal/testdb"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	pool, _, cleanup, err := testdb.Start(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "Testdatenbank:", err)
		os.Exit(1)
	}
	testPool = pool
	code := m.Run()
	cleanup()
	os.Exit(code)
}
