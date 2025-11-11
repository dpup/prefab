package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/plugins/storage"
	"github.com/dpup/prefab/plugins/storage/storagetests"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestOptions(t *testing.T) {
	t.Run("WithPrefix", func(t *testing.T) {
		s := &store{prefix: "prefab_"}
		WithPrefix("custom_")(s)
		assert.Equal(t, "custom_", s.prefix)
	})

	t.Run("WithSchema", func(t *testing.T) {
		s := &store{schema: "public"}
		WithSchema("custom_schema")(s)
		assert.Equal(t, "custom_schema", s.schema)
	})

	t.Run("WithAutoCreateTables", func(t *testing.T) {
		s := &store{autoCreateTables: true}
		WithAutoCreateTables(false)(s)
		assert.False(t, s.autoCreateTables)

		WithAutoCreateTables(true)(s)
		assert.True(t, s.autoCreateTables)
	})
}

func TestSafeNewErrors(t *testing.T) {
	t.Run("InvalidConnectionString", func(t *testing.T) {
		_, err := SafeNew("://invalid")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "PostgreSQL")
	})

	t.Run("ConnectionFailed", func(t *testing.T) {
		// Use a connection string that will fail to connect
		_, err := SafeNew("postgres://user:pass@localhost:1/nonexistent?connect_timeout=1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to connect to PostgreSQL")
	})
}

func TestTranslateError(t *testing.T) {
	tests := []struct {
		name     string
		input    error
		expected error
	}{
		{
			name:     "NilError",
			input:    nil,
			expected: nil,
		},
		{
			name:     "ErrNoRows",
			input:    sql.ErrNoRows,
			expected: storage.ErrNotFound,
		},
		{
			name:     "UniqueViolation",
			input:    &pq.Error{Code: "23505"},
			expected: storage.ErrAlreadyExists,
		},
		{
			name:     "UniqueConstraintMessage",
			input:    errors.New("violates unique constraint"),
			expected: storage.ErrAlreadyExists,
		},
		{
			name:     "NotNullConstraintMessage",
			input:    errors.New("violates not-null constraint"),
			expected: storage.ErrInvalidModel,
		},
		{
			name:     "NoRowsMessage",
			input:    errors.New("no rows in result set"),
			expected: storage.ErrNotFound,
		},
		{
			name:     "RecordNotFoundMessage",
			input:    errors.New("record not found"),
			expected: storage.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translateError(tt.input)
			if tt.expected == nil {
				assert.NoError(t, result)
			} else {
				require.Error(t, result)
				assert.ErrorIs(t, result, tt.expected)
			}
		})
	}
}

type TestModel struct {
	ID string `json:"id"`
}

func (m TestModel) PK() string {
	return m.ID
}

type FilterModel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (m FilterModel) PK() string {
	return m.ID
}

func TestTableName(t *testing.T) {
	s := &store{
		prefix: "test_",
		schema: "custom_schema",
		tables: map[string]bool{},
	}

	t.Run("DefaultTable", func(t *testing.T) {
		model := TestModel{ID: "1"}
		tableName, isDefault := s.tableName(model)
		assert.True(t, isDefault)
		assert.Equal(t, "custom_schema.test_default", tableName)
	})

	t.Run("DedicatedTable", func(t *testing.T) {
		// Use the actual pluralized name that storage.Name() would return
		modelName := storage.Name(TestModel{})
		s.tables[modelName] = true
		model := TestModel{ID: "1"}
		tableName, isDefault := s.tableName(model)
		assert.False(t, isDefault)
		assert.Equal(t, "custom_schema.test_"+modelName, tableName)
	})
}

// Helper to create a mock store
func newMockStore(t *testing.T) (*store, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	s := &store{
		db:               db,
		prefix:           "test_",
		schema:           "public",
		tables:           map[string]bool{},
		autoCreateTables: false, // Disable auto-creation for mocks
	}
	return s, mock
}

func TestBuildListQuery(t *testing.T) {
	s := &store{
		prefix: "test_",
		schema: "public",
		tables: map[string]bool{},
	}

	t.Run("NoFilter", func(t *testing.T) {
		query, args := s.buildListQuery(TestModel{})
		assert.Contains(t, query, "SELECT value FROM")
		assert.Contains(t, query, "entity_type = $1")
		assert.Len(t, args, 1)
		assert.Equal(t, storage.Name(TestModel{}), args[0])
	})

	t.Run("WithFilter", func(t *testing.T) {
		filter := FilterModel{Name: "test"}

		// Register the table so it uses dedicated table logic
		filterModelName := storage.Name(FilterModel{})
		s.tables[filterModelName] = true

		query, args := s.buildListQuery(filter)
		assert.Contains(t, query, "SELECT value FROM")
		assert.Contains(t, query, "value->>'Name' = $1")
		assert.Len(t, args, 1)
		assert.Equal(t, "test", args[0])
	})
}

func TestCreateWithMock(t *testing.T) {
	s, mock := newMockStore(t)
	defer s.db.Close()

	t.Run("CreateSuccess", func(t *testing.T) {
		model := TestModel{ID: "1"}
		data, _ := json.Marshal(model)

		mock.ExpectBegin()
		mock.ExpectPrepare("INSERT INTO").
			ExpectExec().
			WithArgs("1", storage.Name(model), data).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		err := s.Create(context.Background(), model)
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("CreateConflict", func(t *testing.T) {
		model := TestModel{ID: "1"}
		data, _ := json.Marshal(model)

		mock.ExpectBegin()
		mock.ExpectPrepare("INSERT INTO").
			ExpectExec().
			WithArgs("1", storage.Name(model), data).
			WillReturnError(&pq.Error{Code: "23505"})
		mock.ExpectRollback()

		err := s.Create(context.Background(), model)
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrAlreadyExists)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestReadWithMock(t *testing.T) {
	s, mock := newMockStore(t)
	defer s.db.Close()

	t.Run("ReadSuccess", func(t *testing.T) {
		model := TestModel{ID: "1"}
		data, _ := json.Marshal(model)

		mock.ExpectQuery("SELECT value FROM").
			WithArgs("1", storage.Name(TestModel{})).
			WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(data))

		var result TestModel
		err := s.Read(context.Background(), "1", &result)
		require.NoError(t, err)
		assert.Equal(t, "1", result.ID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ReadNotFound", func(t *testing.T) {
		mock.ExpectQuery("SELECT value FROM").
			WithArgs("1", storage.Name(TestModel{})).
			WillReturnError(sql.ErrNoRows)

		var result TestModel
		err := s.Read(context.Background(), "1", &result)
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrNotFound)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ReadNilModel", func(t *testing.T) {
		var result *TestModel
		err := s.Read(context.Background(), "1", result)
		require.Error(t, err)
		assert.ErrorIs(t, err, storage.ErrNilModel)
	})
}

func TestUpdateWithMock(t *testing.T) {
	s, mock := newMockStore(t)
	defer s.db.Close()

	t.Run("UpdateSuccess", func(t *testing.T) {
		model := TestModel{ID: "1"}
		data, _ := json.Marshal(model)

		mock.ExpectBegin()
		mock.ExpectPrepare("UPDATE").
			ExpectExec().
			WithArgs(data, "1", storage.Name(model)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		err := s.Update(context.Background(), model)
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("UpdateNotFound", func(t *testing.T) {
		model := TestModel{ID: "1"}
		data, _ := json.Marshal(model)

		mock.ExpectBegin()
		mock.ExpectPrepare("UPDATE").
			ExpectExec().
			WithArgs(data, "1", storage.Name(model)).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectRollback()

		err := s.Update(context.Background(), model)
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrNotFound)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestUpsertWithMock(t *testing.T) {
	s, mock := newMockStore(t)
	defer s.db.Close()

	model := TestModel{ID: "1"}
	data, _ := json.Marshal(model)

	mock.ExpectBegin()
	mock.ExpectPrepare("INSERT INTO").
		ExpectExec().
		WithArgs("1", storage.Name(model), data).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := s.Upsert(context.Background(), model)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteWithMock(t *testing.T) {
	s, mock := newMockStore(t)
	defer s.db.Close()

	t.Run("DeleteSuccess", func(t *testing.T) {
		model := TestModel{ID: "1"}

		mock.ExpectPrepare("DELETE FROM").
			ExpectExec().
			WithArgs("1", storage.Name(model)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := s.Delete(context.Background(), model)
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("DeleteNotFound", func(t *testing.T) {
		model := TestModel{ID: "1"}

		mock.ExpectPrepare("DELETE FROM").
			ExpectExec().
			WithArgs("1", storage.Name(model)).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := s.Delete(context.Background(), model)
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrNotFound)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestExistsWithMock(t *testing.T) {
	s, mock := newMockStore(t)
	defer s.db.Close()

	t.Run("ExistsTrue", func(t *testing.T) {
		mock.ExpectQuery("SELECT COUNT").
			WithArgs("1", storage.Name(TestModel{})).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		exists, err := s.Exists(context.Background(), "1", &TestModel{})
		require.NoError(t, err)
		assert.True(t, exists)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ExistsFalse", func(t *testing.T) {
		mock.ExpectQuery("SELECT COUNT").
			WithArgs("1", storage.Name(TestModel{})).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		exists, err := s.Exists(context.Background(), "1", &TestModel{})
		require.NoError(t, err)
		assert.False(t, exists)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestListWithMock(t *testing.T) {
	s, mock := newMockStore(t)
	defer s.db.Close()

	t.Run("ListSuccess", func(t *testing.T) {
		model1 := TestModel{ID: "1"}
		model2 := TestModel{ID: "2"}
		data1, _ := json.Marshal(model1)
		data2, _ := json.Marshal(model2)

		mock.ExpectQuery("SELECT value FROM").
			WithArgs(storage.Name(TestModel{})).
			WillReturnRows(sqlmock.NewRows([]string{"value"}).
				AddRow(string(data1)).
				AddRow(string(data2)))

		var results []TestModel
		err := s.List(context.Background(), &results, TestModel{})
		require.NoError(t, err)
		assert.Len(t, results, 2)
		assert.Equal(t, "1", results[0].ID)
		assert.Equal(t, "2", results[1].ID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ListEmpty", func(t *testing.T) {
		mock.ExpectQuery("SELECT value FROM").
			WithArgs(storage.Name(TestModel{})).
			WillReturnRows(sqlmock.NewRows([]string{"value"}))

		var results []TestModel
		err := s.List(context.Background(), &results, TestModel{})
		require.NoError(t, err)
		assert.Empty(t, results)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestInitModelWithMock(t *testing.T) {
	s, mock := newMockStore(t)
	s.autoCreateTables = true
	defer s.db.Close()

	model := TestModel{ID: "1"}

	// Expect schema creation
	mock.ExpectExec("CREATE SCHEMA IF NOT EXISTS").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Expect table creation
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Expect index creation
	mock.ExpectExec("CREATE INDEX IF NOT EXISTS").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Expect function creation
	mock.ExpectExec("CREATE OR REPLACE FUNCTION").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Expect trigger creation
	mock.ExpectExec("DROP TRIGGER IF EXISTS").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := s.InitModel(model)
	require.NoError(t, err)
	assert.True(t, s.tables[storage.Name(model)])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEnsureDefaultTableWithMock(t *testing.T) {
	s, mock := newMockStore(t)
	defer s.db.Close()

	t.Run("Success", func(t *testing.T) {
		// Expect schema creation
		mock.ExpectExec("CREATE SCHEMA IF NOT EXISTS").
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Expect default table creation
		mock.ExpectExec("CREATE TABLE IF NOT EXISTS").
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Expect entity_type index creation
		mock.ExpectExec("CREATE INDEX IF NOT EXISTS.*entity_type").
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Expect GIN index creation on JSONB
		mock.ExpectExec("CREATE INDEX IF NOT EXISTS.*USING GIN").
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Expect timestamp function creation
		mock.ExpectExec("CREATE OR REPLACE FUNCTION").
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Expect trigger drop and creation
		mock.ExpectExec("DROP TRIGGER IF EXISTS").
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := s.ensureDefaultTable()
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("SchemaCreationError", func(t *testing.T) {
		mock.ExpectExec("CREATE SCHEMA IF NOT EXISTS").
			WillReturnError(errors.New("permission denied"))

		err := s.ensureDefaultTable()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create schema")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("TableCreationError", func(t *testing.T) {
		mock.ExpectExec("CREATE SCHEMA IF NOT EXISTS").
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("CREATE TABLE IF NOT EXISTS").
			WillReturnError(errors.New("table already exists"))

		err := s.ensureDefaultTable()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create default table")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("IndexCreationError", func(t *testing.T) {
		mock.ExpectExec("CREATE SCHEMA IF NOT EXISTS").
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("CREATE TABLE IF NOT EXISTS").
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec("CREATE INDEX IF NOT EXISTS.*entity_type").
			WillReturnError(errors.New("index error"))

		err := s.ensureDefaultTable()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create entity_type index")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
