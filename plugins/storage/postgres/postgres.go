// Package postgres provides a PostgreSQL implementation of storage.Store
// interface. It stores model data in PostgreSQL tables using JSONB columns for
// efficient storage and querying.
//
// This implementation is compatible with all the operations defined in the
// storage.Store interface and passes all the standard tests in the storagetests
// package.
//
// Examples:
//
//	store := postgres.New(
//		"postgres://user:password@localhost/dbname?sslmode=disable",
//		postgres.WithPrefix("prefab_"),
//	)
//
//	// Use with the storage plugin
//	server := prefab.New(
//		prefab.WithPlugin(storage.Plugin(store)),
//		// Other plugins...
//	)
//
package postgres

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/plugins/storage"
	"github.com/lib/pq"
	_ "github.com/lib/pq" // Register postgres driver
)

// Option is a functional option for configuring the store.
type Option func(*store)

// WithPrefix overrides the default prefix for table names.
func WithPrefix(prefix string) Option {
	return func(s *store) {
		s.prefix = prefix
	}
}

// WithSchema sets the PostgreSQL schema to use for tables.
// By default, tables are created in the public schema.
func WithSchema(schema string) Option {
	return func(s *store) {
		s.schema = schema
	}
}

// WithAutoCreateTables controls whether tables, indexes, and triggers are 
// automatically created. Set to false in production environments where
// database migrations are managed separately.
func WithAutoCreateTables(autoCreate bool) Option {
	return func(s *store) {
		s.autoCreateTables = autoCreate
	}
}

// New returns a store that provides PostgreSQL backed storage, the table will be
// created optimistically on initialization. Any errors are considered
// non-recoverable and will panic, unless SafeNew is used instead.
func New(connString string, opts ...Option) storage.Store {
	store, err := SafeNew(connString, opts...)
	if err != nil {
		panic(err.Error())
	}
	return store
}

// SafeNew is like New but returns errors instead of panicking.
// This is useful for testing or when you want to handle connection errors.
func SafeNew(connString string, opts ...Option) (storage.Store, error) {
	db, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, fmt.Errorf("failed to open PostgreSQL connection: %w", err)
	}
	
	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}
	
	s := &store{
		db:              db,
		prefix:          "prefab_",
		schema:          "public",
		tables:          map[string]bool{},
		autoCreateTables: true, // Default to automatically creating tables
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.autoCreateTables {
		if err := s.ensureDefaultTable(); err != nil {
			db.Close()
			return nil, err
		}
	}
	return s, nil
}

type store struct {
	db              *sql.DB
	prefix          string
	schema          string
	tables          map[string]bool
	autoCreateTables bool
}

// From ModelInitializer interface. Sets up dedicated table for the model.
func (s *store) InitModel(model storage.Model) error {
	name := storage.Name(model)
	s.tables[name] = true
	
	// Only create the table if auto-creation is enabled
	if s.autoCreateTables {
		return s.ensureTable(name)
	}
	return nil
}

func (s *store) Create(models ...storage.Model) error {
	return s.insert(false, models...)
}

func (s *store) Read(id string, model storage.Model) error {
	if err := storage.ValidateReceiver(model); err != nil {
		return err
	}

	var query string
	var args []interface{}
	
	if tableName, isDefault := s.tableName(model); isDefault {
		query = "SELECT value FROM " + tableName + " WHERE id = $1 AND entity_type = $2"
		args = []interface{}{id, storage.Name(model)}
	} else {
		query = "SELECT value FROM " + tableName + " WHERE id = $1"
		args = []interface{}{id}
	}
	
	row := s.db.QueryRow(query, args...)

	var value []byte
	err := row.Scan(&value)
	if err != nil {
		return translateError(err)
	}

	return errors.MaybeWrap(json.Unmarshal(value, model), 0)
}

func (s *store) Update(models ...storage.Model) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	for _, model := range models {
		id := model.PK()
		entityType := storage.Name(model)
		value, err := json.Marshal(model)
		if err != nil {
			tx.Rollback()
			return errors.Mark(storage.ErrInvalidModel, 0).Append(err.Error())
		}

		var res sql.Result
		if tableName, isDefault := s.tableName(model); isDefault {
			res, err = prepareAndExec(tx,
				"UPDATE "+tableName+" SET value = $1, updated_at = NOW() WHERE id = $2 AND entity_type = $3",
				value, id, entityType)
		} else {
			res, err = prepareAndExec(tx,
				"UPDATE "+tableName+" SET value = $1, updated_at = NOW() WHERE id = $2",
				value, id)
		}
		if err != nil {
			tx.Rollback()
			return translateError(err)
		}
		if i, err := res.RowsAffected(); i == 0 || err != nil {
			tx.Rollback()
			return errors.Wrap(storage.ErrNotFound, 0)
		}
	}

	if err := tx.Commit(); err != nil {
		tx.Rollback()
		return translateError(err)
	}

	return nil
}

func (s *store) Upsert(models ...storage.Model) error {
	return s.insert(true, models...)
}

func (s *store) Delete(model storage.Model) error {
	var query string
	var args []interface{}
	
	if tableName, isDefault := s.tableName(model); isDefault {
		query = "DELETE FROM " + tableName + " WHERE id = $1 AND entity_type = $2"
		args = []interface{}{model.PK(), storage.Name(model)}
	} else {
		query = "DELETE FROM " + tableName + " WHERE id = $1"
		args = []interface{}{model.PK()}
	}
	
	stmt, err := s.db.Prepare(query)
	if err != nil {
		return translateError(err)
	}
	defer stmt.Close()

	res, err := stmt.Exec(args...)
	if err != nil {
		return translateError(err)
	}
	
	if i, errAff := res.RowsAffected(); i == 0 || errAff != nil {
		return errors.Wrap(storage.ErrNotFound, 0)
	}
	
	return nil
}

func (s *store) List(models any, filter storage.Model) error {
	modelsVal := reflect.ValueOf(models)
	if modelsVal.Kind() != reflect.Ptr || modelsVal.Elem().Kind() != reflect.Slice {
		return storage.ErrSliceRequired
	}
	sliceVal := modelsVal.Elem()
	elemType := sliceVal.Type().Elem()
	if elemType != reflect.TypeOf(filter) {
		return storage.ErrTypeMismatch
	}

	query, args := s.buildListQuery(filter)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return translateError(err)
	}
	defer rows.Close()

	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return translateError(err)
		}

		newElemPtr := reflect.New(elemType)
		newElem := newElemPtr.Elem()
		err := json.Unmarshal([]byte(value), newElem.Addr().Interface())
		if err != nil {
			return errors.Mark(storage.ErrInvalidModel, 0).
				Append(err.Error()).
				Append(fmt.Sprintf("<%v>", value))
		}

		sliceVal.Set(reflect.Append(sliceVal, newElem))
	}

	if err := rows.Err(); err != nil {
		return translateError(err)
	}

	return nil
}

func (s *store) Exists(id string, model storage.Model) (bool, error) {
	var query string
	var args []interface{}
	
	if tableName, isDefault := s.tableName(model); isDefault {
		query = "SELECT COUNT(*) FROM " + tableName + " WHERE id = $1 AND entity_type = $2"
		args = []interface{}{id, storage.Name(model)}
	} else {
		query = "SELECT COUNT(*) FROM " + tableName + " WHERE id = $1"
		args = []interface{}{id}
	}

	var count int
	err := s.db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return false, translateError(err)
	}
	
	return count > 0, nil
}

func (s *store) tableName(model storage.Model) (string, bool) {
	name := storage.Name(model)
	if _, ok := s.tables[name]; !ok {
		return s.schema + "." + s.prefix + "default", true
	}
	return s.schema + "." + s.prefix + name, false
}

func (s *store) insert(upsert bool, models ...storage.Model) error {
	tx, err := s.db.Begin()
	if err != nil {
		return translateError(err)
	}

	for _, model := range models {
		id := model.PK()
		entityType := storage.Name(model)
		value, err := json.Marshal(model)
		if err != nil {
			tx.Rollback()
			return errors.Errorf("%w: %s", storage.ErrInvalidModel, err)
		}

		var query string
		var args []interface{}
		
		if tableName, isDefault := s.tableName(model); isDefault {
			if upsert {
				query = `
					INSERT INTO ` + tableName + ` (id, entity_type, value, created_at, updated_at) 
					VALUES ($1, $2, $3, NOW(), NOW())
					ON CONFLICT (id, entity_type) DO UPDATE SET 
					value = $3, updated_at = NOW()
				`
			} else {
				query = `
					INSERT INTO ` + tableName + ` (id, entity_type, value, created_at, updated_at) 
					VALUES ($1, $2, $3, NOW(), NOW())
				`
			}
			args = []interface{}{id, entityType, value}
		} else {
			if upsert {
				query = `
					INSERT INTO ` + tableName + ` (id, value, created_at, updated_at) 
					VALUES ($1, $2, NOW(), NOW())
					ON CONFLICT (id) DO UPDATE SET 
					value = $2, updated_at = NOW()
				`
			} else {
				query = `
					INSERT INTO ` + tableName + ` (id, value, created_at, updated_at) 
					VALUES ($1, $2, NOW(), NOW())
				`
			}
			args = []interface{}{id, value}
		}
		
		_, err = prepareAndExec(tx, query, args...)
		if err != nil {
			tx.Rollback()
			return translateError(err)
		}
	}
	
	if err := tx.Commit(); err != nil {
		tx.Rollback()
		return translateError(err)
	}

	return nil
}

func (s *store) ensureDefaultTable() error {
	// First ensure schema exists
	_, err := s.db.Exec(`CREATE SCHEMA IF NOT EXISTS ` + s.schema + `;`)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Create default table with proper constraints and types
	_, err = s.db.Exec(`CREATE TABLE IF NOT EXISTS ` + s.schema + `.` + s.prefix + `default (
		id TEXT NOT NULL,
		entity_type TEXT NOT NULL,
		value JSONB NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
		PRIMARY KEY (id, entity_type)
	);`)
	if err != nil {
		return fmt.Errorf("failed to create default table: %w", err)
	}

	// Create index on entity_type
	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_` + s.prefix + `default_entity_type 
		ON ` + s.schema + `.` + s.prefix + `default(entity_type);`)
	if err != nil {
		return fmt.Errorf("failed to create entity_type index: %w", err)
	}

	// Create GIN index on JSONB value
	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_` + s.prefix + `default_value 
		ON ` + s.schema + `.` + s.prefix + `default USING GIN (value jsonb_path_ops);`)
	if err != nil {
		return fmt.Errorf("failed to create JSONB index: %w", err)
	}

	// Create update timestamp function if it doesn't exist
	_, err = s.db.Exec(`
		CREATE OR REPLACE FUNCTION ` + s.schema + `.update_timestamp()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = NOW();
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
	`)
	if err != nil {
		return fmt.Errorf("failed to create timestamp function: %w", err)
	}

	// Create trigger for automatic timestamp updates
	_, err = s.db.Exec(`
		DROP TRIGGER IF EXISTS update_` + s.prefix + `default_timestamp 
		ON ` + s.schema + `.` + s.prefix + `default;
		
		CREATE TRIGGER update_` + s.prefix + `default_timestamp
		BEFORE UPDATE ON ` + s.schema + `.` + s.prefix + `default
		FOR EACH ROW
		EXECUTE FUNCTION ` + s.schema + `.update_timestamp();
	`)
	if err != nil {
		return fmt.Errorf("failed to create timestamp trigger: %w", err)
	}
	
	return nil
}

func (s *store) ensureTable(tableName string) error {
	// First ensure schema exists
	_, err := s.db.Exec(`CREATE SCHEMA IF NOT EXISTS ` + s.schema + `;`)
	if err != nil {
		return errors.Errorf("failed to create schema: %w", err)
	}

	// Create model-specific table with proper constraints and types
	_, err = s.db.Exec(`CREATE TABLE IF NOT EXISTS ` + s.schema + `.` + s.prefix + tableName + ` (
		id TEXT NOT NULL PRIMARY KEY,
		value JSONB NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);`)
	if err != nil {
		return errors.Errorf("failed to create table [%s]: %w", tableName, err)
	}

	// Create GIN index on JSONB value
	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_` + s.prefix + tableName + `_value 
		ON ` + s.schema + `.` + s.prefix + tableName + ` USING GIN (value jsonb_path_ops);`)
	if err != nil {
		return errors.Errorf("failed to create JSONB index for [%s]: %w", tableName, err)
	}

	// Ensure the update timestamp function exists
	_, err = s.db.Exec(`
		CREATE OR REPLACE FUNCTION ` + s.schema + `.update_timestamp()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = NOW();
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
	`)
	if err != nil {
		return errors.Errorf("failed to create timestamp function: %w", err)
	}

	// Create trigger for automatic timestamp updates
	_, err = s.db.Exec(`
		DROP TRIGGER IF EXISTS update_` + s.prefix + tableName + `_timestamp 
		ON ` + s.schema + `.` + s.prefix + tableName + `;
		
		CREATE TRIGGER update_` + s.prefix + tableName + `_timestamp
		BEFORE UPDATE ON ` + s.schema + `.` + s.prefix + tableName + `
		FOR EACH ROW
		EXECUTE FUNCTION ` + s.schema + `.update_timestamp();
	`)
	if err != nil {
		return errors.Errorf("failed to create timestamp trigger for [%s]: %w", tableName, err)
	}

	return nil
}

func (s *store) buildListQuery(model storage.Model) (string, []interface{}) {
	tableName, isDefault := s.tableName(model)
	modelType := reflect.TypeOf(model)
	modelValue := reflect.ValueOf(model)

	var whereClauses []string
	var args []interface{}
	paramIdx := 1

	if isDefault {
		whereClauses = append(whereClauses, fmt.Sprintf("entity_type = $%d", paramIdx))
		args = append(args, storage.Name(model))
		paramIdx++
	}

	for i := 0; i < modelType.NumField(); i++ {
		field := modelValue.Field(i)
		typeField := modelType.Field(i)

		// Only include fields that are non-nil pointers or are non-zero values.
		if (field.Kind() == reflect.Ptr && !field.IsNil()) || (!field.IsZero() && field.Kind() != reflect.Ptr) {
			// In PostgreSQL, we use the JSON path notation for JSONB fields
			w := fmt.Sprintf("value->>'%s' = $%d", typeField.Name, paramIdx)
			whereClauses = append(whereClauses, w)
			
			// For postgres, we need to convert the value to string for JSONB comparison
			var paramValue interface{}
			if field.Kind() == reflect.Ptr {
				paramValue = fmt.Sprintf("%v", reflect.Indirect(field).Interface())
			} else {
				paramValue = fmt.Sprintf("%v", field.Interface())
			}
			
			args = append(args, paramValue)
			paramIdx++
		}
	}

	query := "SELECT value FROM " + tableName
	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}
	
	return query, args
}

func translateError(err error) error {
	if err == nil {
		return nil
	}
	
	if errors.Is(err, sql.ErrNoRows) {
		return errors.Wrap(storage.ErrNotFound, 0)
	}
	
	if pqErr, ok := err.(*pq.Error); ok {
		switch pqErr.Code {
		case "23505": // unique_violation
			return errors.Wrap(storage.ErrAlreadyExists, 0)
		}
	}
	
	// If the error message contains specific phrases, map to storage errors
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "violates unique constraint"):
		return errors.Wrap(storage.ErrAlreadyExists, 0)
	case strings.Contains(errMsg, "violates not-null constraint"):
		return errors.Wrap(storage.ErrInvalidModel, 0)
	case strings.Contains(errMsg, "no rows in result set"):
		return errors.Wrap(storage.ErrNotFound, 0)
	case strings.Contains(errMsg, "record not found"):
		return errors.Wrap(storage.ErrNotFound, 0)
	}
	
	// Default: wrap the error
	return errors.MaybeWrap(err, 0)
}

func prepareAndExec(tx *sql.Tx, query string, args ...interface{}) (sql.Result, error) {
	stmt, err := tx.Prepare(query)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	defer stmt.Close()
	return stmt.Exec(args...)
}