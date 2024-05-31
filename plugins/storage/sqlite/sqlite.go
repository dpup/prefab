// Package sqlite provides a SQLite implementation of storage.Store
// interface.
//
// Examples:
//
//	store := sqlitestore.New(
//		"file:test.s3db?_auth&_auth_user=admin&_auth_pass=admin",
//		sqlitestore.WithTableName("plugin_store"),
//	)
//
//	store := sqlitestore.New(":memory:")
//
//nolint:gosec // Reports on G202. SQL string concat used to parameterize table.
package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/plugins/storage"

	"github.com/mattn/go-sqlite3"
)

// Option is a functional option for configuring the store.
type Option func(*store)

// WithPrefix overides the default prefix for table names.
func WithPrefix(prefix string) Option {
	return func(s *store) {
		s.prefix = prefix
	}
}

// New returns a store that provides sqlite backed storage, the table will be
// created optimistically on initialization. Any errors are considered
// non-recoverable and will panic.
func New(conn string, opts ...Option) storage.Store {
	db, err := sql.Open("sqlite3", conn)
	if err != nil {
		panic("failed to open sqlite connection: " + err.Error())
	}
	s := &store{
		db:     db,
		prefix: "prefab_",
		tables: map[string]bool{},
	}
	for _, opt := range opts {
		opt(s)
	}
	s.ensureDefaultTable()
	return s
}

type store struct {
	db     *sql.DB
	prefix string
	tables map[string]bool
}

// From ModelInitializer interface. Sets up dedicated for the model.
func (s *store) InitModel(model storage.Model) error {
	name := storage.Name(model)
	s.tables[name] = true
	return s.ensureTable(name)
}

func (s *store) Create(models ...storage.Model) error {
	return s.insert(false, models...)
}

func (s *store) Read(id string, model storage.Model) error {
	if err := storage.ValidateReceiver(model); err != nil {
		return err
	}

	var query string
	if tableName, isDefault := s.tableName(model); isDefault {
		query = "SELECT value FROM " + tableName + " WHERE id = ? AND entity_type = ?"
	} else {
		query = "SELECT value FROM " + tableName + " WHERE id = ?"
	}
	row := s.db.QueryRow(query, id, storage.Name(model))

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
				"UPDATE "+tableName+" SET value = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND entity_type = ?",
				value, id, entityType)
		} else {
			res, err = prepareAndExec(tx,
				"UPDATE "+tableName+" SET value = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
				value, id)
		}
		if err != nil {
			tx.Rollback()
			return translateError(err)
		}
		if i, err := res.RowsAffected(); i == 0 || err != nil {
			tx.Rollback()
			return errors.Mark(storage.ErrNotFound, 0)
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
	var params []any
	var stmt *sql.Stmt
	var err error
	if tableName, isDefault := s.tableName(model); isDefault {
		stmt, err = s.db.Prepare("DELETE FROM " + tableName + " WHERE id = ? AND entity_type = ?")
		params = []any{model.PK(), storage.Name(model)}
	} else {
		stmt, err = s.db.Prepare("DELETE FROM " + tableName + " WHERE id = ?")
		params = []any{model.PK()}
	}
	if err != nil {
		return translateError(err)
	}
	defer stmt.Close()

	res, err := stmt.Exec(params...)
	if err != nil {
		return translateError(err)
	}
	if i, err := res.RowsAffected(); i == 0 || err != nil {
		return errors.Mark(storage.ErrNotFound, 0)
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
	if tableName, isDefault := s.tableName(model); isDefault {
		query = "SELECT COUNT(*) FROM " + tableName + " WHERE id = ? AND entity_type = ?"
	} else {
		query = "SELECT COUNT(*) FROM " + tableName + " WHERE id = ?"
	}

	var value int
	err := s.db.QueryRow(query, id, storage.Name(model)).Scan(&value)
	if err != nil {
		return false, translateError(err)
	}
	return value > 0, nil
}

func (s *store) tableName(model storage.Model) (string, bool) {
	name := storage.Name(model)
	if _, ok := s.tables[name]; !ok {
		return s.prefix + "default", true
	}
	return s.prefix + name, false
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

		if tableName, isDefault := s.tableName(model); isDefault {
			query := `INSERT INTO ` + tableName + ` (id, entity_type, value, created_at, updated_at) 
				VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`
			if upsert {
				query += `
					ON CONFLICT(id, entity_type) DO UPDATE SET 
					value = excluded.value, updated_at = CURRENT_TIMESTAMP`
			}
			_, err = prepareAndExec(tx, query, id, entityType, value)
		} else {
			query := `INSERT INTO ` + tableName + ` (id, value, created_at, updated_at) 
				VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`
			if upsert {
				query += `
					ON CONFLICT(id) DO UPDATE SET 
					value = excluded.value, updated_at = CURRENT_TIMESTAMP`
			}
			_, err = prepareAndExec(tx, query, id, value)
		}
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

func (s *store) ensureDefaultTable() {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS ` + s.prefix + `default (
		id TEXT,
		entity_type TEXT,
		value BLOB,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (id, entity_type)
	);`)
	if err != nil {
		panic("failed to create default table: " + err.Error())
	}
}

func (s *store) ensureTable(tableName string) error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS ` + s.prefix + tableName + ` (
		id TEXT,
		value BLOB,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (id)
	);`)
	if err != nil {
		return errors.Errorf("failed to create table [%s]: %w", tableName, err)
	}
	return nil
}

func (s *store) buildListQuery(model storage.Model) (string, []any) {
	tableName, isDefault := s.tableName(model)
	filterValue := reflect.ValueOf(model)

	var whereClauses []string
	var params []interface{}

	if isDefault {
		whereClauses = append(whereClauses, "entity_type = ?")
		params = append(params, storage.Name(model))
	}

	for i := range filterValue.NumField() {
		field := filterValue.Field(i)
		typeField := filterValue.Type().Field(i)

		// Only include fields that are non-nil pointers or are non-zero values.
		if (field.Kind() == reflect.Ptr && !field.IsNil()) || (!field.IsZero() && field.Kind() != reflect.Ptr) {
			w := fmt.Sprintf("json_extract(value, '$.%s') = ?", typeField.Name)
			whereClauses = append(whereClauses, w)
			params = append(params, field.Interface())
		}
	}

	query := "SELECT value FROM " + tableName
	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}
	return query, params
}

func translateError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return errors.Mark(storage.ErrNotFound, 0)
	}
	var sqlErr sqlite3.Error
	if errors.As(err, &sqlErr) {
		switch sqlErr.Code {
		case sqlite3.ErrNotFound:
			return errors.Mark(storage.ErrNotFound, 0)
		case sqlite3.ErrConstraint:
			return errors.Mark(storage.ErrAlreadyExists, 0)
		}
	}
	return errors.MaybeWrap(err, 0)
}

func prepareAndExec(tx *sql.Tx, query string, params ...any) (sql.Result, error) {
	stmt, err := tx.Prepare(query)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	defer stmt.Close()
	return stmt.Exec(params...)
}
