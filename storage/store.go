package storage

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// Returned when a record does not exist.
	ErrNotFound = status.Error(codes.NotFound, "record not found")

	// Returned when a record conficts with an existing key.
	ErrAlreadyExists = status.Error(codes.AlreadyExists, "primary key already exists")

	// Returned when List is called with a non-slice.
	ErrSliceRequired = status.Error(codes.InvalidArgument, "pointer slice required")

	// Returned when a store can not marshal/unmarshal a model.
	ErrInvalidModel = status.Error(codes.InvalidArgument, "invalid model")

	// Returned when List is called with a filter and slice of mismatching types.
	ErrTypeMismatch = status.Error(codes.InvalidArgument, "type mismatch")

	// Returned when a store is passed an uninitialized pointer.
	ErrNilModel = status.Error(codes.InvalidArgument, "uninitialized pointer passed as model")
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

// Optional interface that stores can implement in order to support per-model
// configuration — for example table per model in SQL databases.
type ModelInitializer interface {
	// InitModel is called by a plugin or application to initialize a model
	// before it is used. Stores will still work, without initialization, however
	// data will be stored in a shared table.
	InitModel(model Model) error
}
