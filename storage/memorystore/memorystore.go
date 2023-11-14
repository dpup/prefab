// Package memory implements storage.Store in a purely in-memory manner.
package memorystore

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"sync"

	"github.com/dpup/prefab/storage"
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

func (s *store) Put(models ...storage.Model) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range models {
		n := storage.Name(m)
		if s.data[n] == nil {
			s.data[n] = map[string][]byte{}
		}
		jsonBytes, err := json.Marshal(m)
		if err != nil {
			return err
		}
		s.data[n][m.PK()] = jsonBytes
	}
	return nil
}

func (s *store) Get(id string, model storage.Model) error {
	if model == nil || (reflect.ValueOf(model).Kind() == reflect.Ptr && reflect.ValueOf(model).IsNil()) {
		return fmt.Errorf("uninitialized pointer passed as model")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	n := storage.Name(model)
	if s.data[n] == nil {
		return storage.ErrNotFound
	}
	if s.data[n][id] == nil {
		return storage.ErrNotFound
	}
	err := json.Unmarshal(s.data[n][id], model)
	if err != nil {
		return err
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

func (s *store) Delete(model storage.Model) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	n := storage.Name(model)
	id := model.PK()
	if s.data[n] == nil {
		return storage.ErrNotFound
	}
	if s.data[n][id] == nil {
		return storage.ErrNotFound
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
		return storage.ErrSliceRequired
	}

	sliceVal := modelsVal.Elem()
	elemType := sliceVal.Type().Elem()
	if elemType != reflect.TypeOf(filter) {
		return storage.ErrTypeMismatch
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
		if err := s.Get(pk, newElemPtr.Interface().(storage.Model)); err != nil {
			return err
		}
		// Skip if any non-zero field in filter differs from the corresponding field in model.
		skip := false
		for i := 0; i < newElem.NumField(); i++ {
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

// shouldFilter returns true for non-zero values and non-nil pointers.
func shouldFilter(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		return !v.IsNil()
	default:
		return !reflect.DeepEqual(v.Interface(), reflect.Zero(v.Type()).Interface())
	}
}
