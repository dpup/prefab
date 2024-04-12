package storage

import (
	"errors"
)

var (
	// Returned when a record does not exist.
	ErrNotFound = errors.New("record not found")

	// Returned when a record conficts with an existing key.
	ErrAlreadyExists = errors.New("primary key already exists")

	// Returned when List is called with a non-slice.
	ErrSliceRequired = errors.New("pointer slice required")

	// Returned when List is called with a filter and slice of mismatching types.
	ErrTypeMismatch = errors.New("type mismatch")
)

// Store offers a basic CRUUDLE (Create Read Update Upsert Delete List Exists)
// interface that allows prefab plugins to persist data.
type Store interface {
	// Create multiple entities.
	Create(models ...Model) error

	// Read a record with the given id.
	Read(id string, model Model) error

	// Update multiple entities.
	Update(models ...Model) error

	// Update or insert multiple entities.
	Upsert(models ...Model) error

	// TODO: Patch / UpdatePartial

	// Delete a record. Only the primary key needs to be populated.
	Delete(model Model) error

	// List populates the slice of models with records that have fields which
	// match the fields of filter. Zero-value fields will be ignored, unless the
	// field is a pointer.
	List(models any, filter Model) error

	// Exists returns true if a record with the given id exists.
	Exists(id string, model Model) (bool, error)
}
