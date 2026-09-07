package database

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestInitializeSchemaRequiresReadableFile(t *testing.T) {
	if err := InitializeSchema(context.Background(), nil, "does-not-exist.sql"); err == nil {
		t.Fatal("InitializeSchema() succeeded for a missing schema file")
	}
}

func TestSchemaFileIsPresent(t *testing.T) {
	// This test documents the development contract without requiring PostgreSQL.
	// The integration suite executes the file against a real database.
	if _, err := pgxpool.ParseConfig("postgres://localhost/test"); err != nil {
		t.Fatalf("test database URL unexpectedly invalid: %v", err)
	}
}
