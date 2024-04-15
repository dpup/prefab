package sqlitestore

import (
	"reflect"
	"testing"

	"github.com/dpup/prefab/storage"
	"github.com/dpup/prefab/storage/storagetests"
)

func TestSqliteStore(t *testing.T) {
	storagetests.Run(t, func() storage.Store {
		return New(":memory:")
	})
}

func TestSqliteStore_withPrefixAndDedicatedTable(t *testing.T) {
	storagetests.Run(t, func() storage.Store {
		s := New(":memory:", WithPrefix("prefix_")).(*store)
		err := s.InitModel(storagetests.Fruit{})
		if err != nil {
			t.Fatal(err)
		}
		return s
	})
}

type Vehicle struct {
	ID     string
	Type   string
	Wheels int
	Mods   *string
}

func (v Vehicle) PK() string {
	return v.ID
}

type Animal struct {
	ID   string
	Type string
	Legs int
}

func (v Animal) PK() string {
	return v.ID
}

func TestBuildListQuery(t *testing.T) {
	emptyString := ""
	tests := []struct {
		name   string
		filter storage.Model
		query  string
		params []any
	}{
		{
			"empty",
			Vehicle{},
			"SELECT value FROM custom_default WHERE entity_type = ?",
			[]any{"vehicles"},
		},
		{
			"single field filter",
			Vehicle{Type: "car"},
			"SELECT value FROM custom_default WHERE entity_type = ? AND json_extract(value, '$.Type') = ?",
			[]any{"vehicles", "car"},
		},
		{
			"two field filter",
			Vehicle{Type: "car", Wheels: 4},
			"SELECT value FROM custom_default WHERE entity_type = ? AND json_extract(value, '$.Type') = ? AND json_extract(value, '$.Wheels') = ?",
			[]any{"vehicles", "car", 4},
		},
		{
			"zero pointer filter",
			Vehicle{Mods: &emptyString},
			"SELECT value FROM custom_default WHERE entity_type = ? AND json_extract(value, '$.Mods') = ?",
			[]any{"vehicles", &emptyString},
		},
		{
			"dedicated table",
			Animal{Legs: 3},
			"SELECT value FROM custom_animals WHERE json_extract(value, '$.Legs') = ?",
			[]any{3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(":memory:", WithPrefix("custom_")).(*store)
			s.InitModel(Animal{})
			query, params := s.buildListQuery(tt.filter)
			if query != tt.query {
				t.Errorf("buildListQuery() query = %v, want %v", query, tt.query)
			}
			if !reflect.DeepEqual(params, tt.params) {
				t.Errorf("buildListQuery() params = %v, want %v", params, tt.params)
			}
		})
	}
}
