//go:build integration

package postgres

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool connects to TEST_DATABASE_URL (skipping when unset), resets the
// ondota schema and applies every up migration, so each run starts clean.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := Open(ctx, url)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS ondota CASCADE"); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	files, err := filepath.Glob("../../../../migrations/*.up.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("no migrations found: %v", err)
	}
	sort.Strings(files)
	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("apply %s: %v", filepath.Base(f), err)
		}
	}
	return pool
}

func TestMigrationsAndCheck(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	if err := Check(pool)(ctx); err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	var schema string
	if err := pool.QueryRow(ctx, "SELECT schema_name FROM information_schema.schemata WHERE schema_name = 'ondota'").Scan(&schema); err != nil {
		t.Fatalf("schema ondota missing after migrations: %v", err)
	}
}

func TestOpenFailsOnUnreachableDatabase(t *testing.T) {
	_, err := Open(context.Background(), "postgres://nobody:nothing@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")
	if err == nil {
		t.Fatal("Open() succeeded against an unreachable database")
	}
}
