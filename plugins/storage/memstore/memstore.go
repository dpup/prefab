// Package memstore implements storage.Store in a purely in-memory manner.
package memstore

import (
	"encoding/json"
	"reflect"
	"sort"
	"sync"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/plugins/storage"
)

// New returns a store that provides transient, in-memory storage.
func New() storage.Store {
	return &store{
		data: map[string]map[string][]byte{},
	}
}

type store struct {
	// store[tableName][entityID] = JSON
	data map[string]map[string][]byte
	mu   sync.RWMutex
}

func (s *store) Create(models ...storage.Model) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check that no conflicting model exists.
	for _, m := range models {
		n := storage.Name(m)
		if s.data[n] != nil && s.data[n][m.PK()] != nil {
			return errors.Mark(storage.ErrAlreadyExists, 0)
		}
	}

	// Update the memory store.
	for _, m := range models {
		n := storage.Name(m)
		if s.data[n] == nil {
			s.data[n] = map[string][]byte{}
		}
		jsonBytes, err := json.Marshal(m)
		if err != nil {
			return errors.Mark(storage.ErrInvalidModel, 0).Append(err.Error())
		}
		s.data[n][m.PK()] = jsonBytes
	}
	return nil
}

func (s *store) Read(id string, model storage.Model) error {
	if err := storage.ValidateReceiver(model); err != nil {
		return err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	n := storage.Name(model)
	if s.data[n] == nil {
		return errors.Mark(storage.ErrNotFound, 0)
	}
	if s.data[n][id] == nil {
		return errors.Mark(storage.ErrNotFound, 0)
	}
	err := json.Unmarshal(s.data[n][id], model)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return nil
}

func (s *store) Update(models ...storage.Model) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Try to serialize models first, to keep error cases consistent.
	records := map[string]map[string][]byte{}
	for _, m := range models {
		jsonBytes, err := json.Marshal(m)
		if err != nil {
			return errors.Mark(storage.ErrInvalidModel, 0).Append(err.Error())
		}
		if records[storage.Name(m)] == nil {
			records[storage.Name(m)] = map[string][]byte{}
		}
		records[storage.Name(m)][m.PK()] = jsonBytes
	}

	// Check that all models exist.
	for _, m := range models {
		n := storage.Name(m)
		if s.data[n] == nil {
			return errors.Mark(storage.ErrNotFound, 0)
		}
		if s.data[n][m.PK()] == nil {
			return errors.Mark(storage.ErrNotFound, 0)
		}
	}

	// Update the memory store.
	for n, r := range records {
		for id, jsonBytes := range r {
			s.data[n][id] = jsonBytes
		}
	}

	return nil
}

func (s *store) Upsert(models ...storage.Model) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range models {
		n := storage.Name(m)
		if s.data[n] == nil {
			s.data[n] = map[string][]byte{}
		}
		jsonBytes, err := json.Marshal(m)
		if err != nil {
			return errors.Mark(storage.ErrInvalidModel, 0).Append(err.Error())
		}
		s.data[n][m.PK()] = jsonBytes
	}
	return nil
}

func (s *store) Delete(model storage.Model) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	n := storage.Name(model)
	id := model.PK()
	if s.data[n] == nil {
		return errors.Mark(storage.ErrNotFound, 0)
	}
	if s.data[n][id] == nil {
		return errors.Mark(storage.ErrNotFound, 0)
	}
	delete(s.data[n], id)
	return nil
}

// List always performs a full scan of all items.
func (s *store) List(models interface{}, filter storage.Model) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	modelsVal := reflect.ValueOf(models)
	if modelsVal.Kind() != reflect.Ptr || modelsVal.Elem().Kind() != reflect.Slice {
		return errors.Mark(storage.ErrSliceRequired, 0)
	}

	sliceVal := modelsVal.Elem()
	elemType := sliceVal.Type().Elem()
	if elemType != reflect.TypeOf(filter) {
		return errors.Mark(storage.ErrTypeMismatch, 0)
	}

	n := storage.Name(filter)
	if s.data[n] == nil {
		return nil
	}

	// Return models sorted by primary key.
	pks := make([]string, 0, len(s.data[n]))
	for pk := range s.data[n] {
		pks = append(pks, pk)
	}
	sort.Strings(pks)

	filterValue := reflect.ValueOf(filter)

	// Fetch and filter models.
	for _, pk := range pks {
		newElemPtr := reflect.New(elemType)
		newElem := newElemPtr.Elem()
		if err := s.Read(pk, newElemPtr.Interface().(storage.Model)); err != nil {
			return errors.Wrap(err, 0)
		}
		// Skip if any non-zero field in filter differs from the corresponding field in model.
		skip := false
		for i := range newElem.NumField() {
			if shouldFilter(filterValue.Field(i)) {
				fieldVal := newElem.Field(i).Interface()
				testVal := filterValue.Field(i).Interface()
				if !reflect.DeepEqual(fieldVal, testVal) {
					skip = true
					break
				}
			}
		}
		if !skip {
			sliceVal.Set(reflect.Append(sliceVal, newElem))
		}
	}

	return nil
}

func (s *store) Exists(id string, model storage.Model) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	n := storage.Name(model)
	if s.data[n] == nil {
		return false, nil
	}
	if s.data[n][id] == nil {
		return false, nil
	}
	return true, nil
}

// shouldFilter returns true for non-zero values and non-nil pointers.
func shouldFilter(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		return !v.IsNil()
	default:
		return !reflect.DeepEqual(v.Interface(), reflect.Zero(v.Type()).Interface())
	}
}
