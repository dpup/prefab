package postgres

import (
	"database/sql"
	"os"
	"testing"

	"github.com/dpup/prefab/plugins/storage"
	"github.com/dpup/prefab/plugins/storage/storagetests"
)

func TestPostgresStore(t *testing.T) {
	// Check if tests are explicitly enabled with PG_TEST_DSN
	dsn := os.Getenv("PG_TEST_DSN")
	if dsn == "" {
		t.Skip("PostgreSQL tests skipped. Set PG_TEST_DSN env var to enable.")
	}

	// Try to connect to the database
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("Skipping PostgreSQL tests - could not open connection: %v", err)
		return
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		t.Skipf("Skipping PostgreSQL tests - could not ping database: %v", err)
		return
	}

	// Set up test schema
	_, err = db.Exec("CREATE SCHEMA IF NOT EXISTS test_schema;")
	if err != nil {
		db.Close()
		t.Skipf("Skipping PostgreSQL tests - could not create schema: %v", err)
		return
	}

	// Clean up any existing test tables and recreate the schema
	// This ensures a completely clean slate for each test run
	_, err = db.Exec(`
		DROP SCHEMA IF EXISTS test_schema CASCADE;
		CREATE SCHEMA test_schema;
	`)
	if err != nil {
		db.Close()
		t.Logf("Warning: could not drop test tables: %v", err)
	}

	// Close the setup connection
	db.Close()

	// For each test, we need a fresh database
	storagetests.Run(t, func() storage.Store {
		// Create a fresh connection and drop/recreate the schema for each test
		// This ensures each test starts with a clean slate
		testDB, err := sql.Open("postgres", dsn)
		if err != nil {
			t.Fatalf("Failed to connect to PostgreSQL: %v", err)
		}

		// Clean up any existing test tables and recreate the schema
		_, err = testDB.Exec(`
			DROP SCHEMA IF EXISTS test_schema CASCADE;
			CREATE SCHEMA test_schema;
		`)
		if err != nil {
			testDB.Close()
			t.Fatalf("Failed to reset test schema: %v", err)
		}
		testDB.Close()

		// Now create a fresh store for this test
		store, err := SafeNew(
			dsn,
			WithPrefix("test_"),
			WithSchema("test_schema"),
		)

		if err != nil {
			t.Fatalf("Failed to connect to PostgreSQL: %v", err)
		}

		return store
	})
}
