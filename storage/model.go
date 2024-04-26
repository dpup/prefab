package storage

import (
	"reflect"

	"github.com/dpup/prefab/errors"
	pluralize "github.com/gertd/go-pluralize"
	"github.com/iancoleman/strcase"
)

var (
	pluralizer = pluralize.NewClient()
	modelNames = map[reflect.Type]string{}
)

// Model defines the interface for records which want to be persisted to a
// storage engine.
type Model interface {
	// PK returns the primary key that the record is stored under.
	PK() string
}

// Namer allows Models to override how the table-name is determined, for engines
// which require it.
type Namer interface {
	Name() string
}

// Name returns a pluralied version of the model's name, either derived from the
// struct or from the `Namer` interface.
func Name(m any) string {
	if n, ok := m.(Namer); ok {
		return n.Name()
	}
	t := reflect.TypeOf(m)
	if t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice {
		t = t.Elem()
	}

	if n, ok := modelNames[t]; ok {
		return n
	} else {
		n = pluralizer.Plural(strcase.ToSnake(t.Name()))
		modelNames[t] = n
		return n
	}
}

// ValidateReceiver returns an error if the model is nil or uninitialized.
func ValidateReceiver(model Model) error {
	if model == nil || (reflect.ValueOf(model).Kind() == reflect.Ptr && reflect.ValueOf(model).IsNil()) {
		return errors.Mark(ErrNilModel, 0)
	}
	return nil
}
