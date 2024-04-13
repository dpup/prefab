// Package sqlitestore provides a SQLite implementation of storage.Store
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
package sqlitestore

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/dpup/prefab/storage"

	"github.com/mattn/go-sqlite3"
)

// Option is a functional option for configuring the store.
type Option func(*store)

// WithTableName overides the default table name of "prefab_store".
func WithTableName(tableName string) Option {
	return func(s *store) {
		s.tableName = tableName
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
		db:        db,
		tableName: "prefab_store",
	}
	for _, opt := range opts {
		opt(s)
	}
	s.ensureTable()
	return s
}

type store struct {
	db *sql.DB

	tableName string
}

func (s *store) Create(models ...storage.Model) error {
	tx, err := s.db.Begin()
	if err != nil {
		return translateError(err)
	}

	stmt, err := tx.Prepare("INSERT INTO " + s.tableName + " (id, entity_type, value) VALUES (?, ?, ?)")
	if err != nil {
		tx.Rollback()
		return translateError(err)
	}
	defer stmt.Close()

	for _, model := range models {
		id := model.PK()
		entityType := storage.Name(model)
		value, err := json.Marshal(model)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("%w: %s", storage.ErrInvalidModel, err)
		}
		_, err = stmt.Exec(id, entityType, value)
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

func (s *store) Read(id string, model storage.Model) error {
	if err := storage.ValidateReceiver(model); err != nil {
		return err
	}

	query := "SELECT value FROM " + s.tableName + " WHERE id = ? AND entity_type = ?"
	row := s.db.QueryRow(query, id, storage.Name(model))

	var value []byte
	err := row.Scan(&value)
	if err != nil {
		return translateError(err)
	}

	return json.Unmarshal(value, model)
}

func (s *store) Update(models ...storage.Model) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare("UPDATE " + s.tableName + " SET value = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND entity_type = ?")
	if err != nil {
		tx.Rollback()
		return translateError(err)
	}
	defer stmt.Close()

	for _, model := range models {
		value, err := json.Marshal(model)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("%w: %s", storage.ErrInvalidModel, err)
		}
		id := model.PK()
		entityType := storage.Name(model)
		res, err := stmt.Exec(value, id, entityType)
		if err != nil {
			tx.Rollback()
			return translateError(err)
		}
		if i, err := res.RowsAffected(); i == 0 || err != nil {
			tx.Rollback()
			return storage.ErrNotFound
		}
	}

	if err := tx.Commit(); err != nil {
		tx.Rollback()
		return translateError(err)
	}

	return nil
}

func (s *store) Upsert(models ...storage.Model) error {
	tx, err := s.db.Begin()
	if err != nil {
		return translateError(err)
	}

	stmt, err := tx.Prepare(
		`INSERT INTO ` + s.tableName + ` (id, entity_type, value, created_at, updated_at) 
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) 
		ON CONFLICT(id, entity_type) DO UPDATE SET 
		value = excluded.value, updated_at = CURRENT_TIMESTAMP`)
	if err != nil {
		tx.Rollback()
		return translateError(err)
	}
	defer stmt.Close()

	for _, model := range models {
		id := model.PK()
		entityType := storage.Name(model)
		value, err := json.Marshal(model)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("%w: %s", storage.ErrInvalidModel, err)
		}

		_, err = stmt.Exec(id, entityType, value)
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

func (s *store) Delete(model storage.Model) error {
	id := model.PK()
	entityType := storage.Name(model)

	stmt, err := s.db.Prepare("DELETE FROM " + s.tableName + " WHERE id = ? AND entity_type = ?")
	if err != nil {
		return translateError(err)
	}
	defer stmt.Close()

	res, err := stmt.Exec(id, entityType)
	if err != nil {
		return translateError(err)
	}
	if i, err := res.RowsAffected(); i == 0 || err != nil {
		return storage.ErrNotFound
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
			return fmt.Errorf("%w: %s", storage.ErrInvalidModel, err)
		}

		sliceVal.Set(reflect.Append(sliceVal, newElem))
	}

	if err := rows.Err(); err != nil {
		return translateError(err)
	}

	return nil
}

func (s *store) Exists(id string, model storage.Model) (bool, error) {
	query := "SELECT COUNT(*) FROM " + s.tableName + " WHERE id = ? AND entity_type = ?"
	var value int
	err := s.db.QueryRow(query, id, storage.Name(model)).Scan(&value)
	if err != nil {
		return false, translateError(err)
	}
	return value > 0, nil
}

func (s *store) ensureTable() {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS ` + s.tableName + ` (
		id TEXT,
		entity_type TEXT,
		value BLOB,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (id, entity_type)
	);`)
	if err != nil {
		panic("failed to create table: " + err.Error())
	}
}

func (s *store) buildListQuery(model storage.Model) (string, []any) {
	filterValue := reflect.ValueOf(model)

	var whereClauses []string
	var params []interface{}
	entityType := storage.Name(model)
	whereClauses = append(whereClauses, "entity_type = ?")
	params = append(params, entityType)

	for i := 0; i < filterValue.NumField(); i++ {
		field := filterValue.Field(i)
		typeField := filterValue.Type().Field(i)

		// Only include fields that are non-nil pointers or are non-zero values.
		if (field.Kind() == reflect.Ptr && !field.IsNil()) || (!field.IsZero() && field.Kind() != reflect.Ptr) {
			w := fmt.Sprintf("json_extract(value, '$.%s') = ?", typeField.Name)
			whereClauses = append(whereClauses, w)
			params = append(params, field.Interface())
		}
	}

	whereClause := "WHERE " + strings.Join(whereClauses, " AND ")
	query := fmt.Sprintf("SELECT value FROM %s %s", s.tableName, whereClause)
	return query, params
}

func translateError(err error) error {
	if err == sql.ErrNoRows {
		return storage.ErrNotFound
	}
	if sqlErr, ok := err.(sqlite3.Error); ok {
		switch sqlErr.Code {
		case sqlite3.ErrNotFound:
			return storage.ErrNotFound
		case sqlite3.ErrConstraint:
			return storage.ErrAlreadyExists
		}
	}
	return err
}
