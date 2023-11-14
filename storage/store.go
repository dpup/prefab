package storage

import "errors"

var (
	// Returned when a record does not exist.
	ErrNotFound = errors.New("record not found")

	// Returned when List is called with a non-slice.
	ErrSliceRequired = errors.New("pointer slice required")

	// Returned when List is called with a filter and slice of mismatching types.
	ErrTypeMismatch = errors.New("type mismatch")
)

// Store offers a basic storage interface that allows prefab plugins to persist
// data.
type Store interface {
	// Put multiple entities.
	Put(models ...Model) error

	// Get a record with the given id.
	Get(id string, model Model) error

	// Exists returns true if a record with the given id exists.
	Exists(id string, model Model) (bool, error)

	// Delete a record. Only the primary key needs to be populated.
	Delete(model Model) error

	// List populates the slice of models with records that have fields which
	// match the fields of filter. Zero-value fields will be ignored, unless the
	// field is a pointer.
	List(models any, filter Model) error
}
