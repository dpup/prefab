// Package storagetests provides common acceptance tests for storage.Store
// implementations.
package storagetests

import (
	"testing"

	"github.com/dpup/prefab/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Color int

const (
	ColorRed    Color = 1
	ColorGreen  Color = 2
	ColorOrange Color = 3
	ColorYellow Color = 4
	ColorBlue   Color = 5
	ColorPurple Color = 6
)

type Fruit struct {
	ID    string
	Name  string
	Color Color
	Count *int // Ptr fields allow filtering on zero values.
}

func (f Fruit) PK() string {
	return f.ID
}

type Planet struct {
	ID   string
	Name string
}

func (p Planet) PK() string {
	return p.ID
}

type BadModel struct {
	ID    string
	Cycle *BadModel
}

func (b BadModel) PK() string {
	return b.ID
}

func pint(i int) *int {
	return &i
}

func Run(t *testing.T, newStore func() storage.Store) {

	t.Run("TestCreateReadRoundTrip", func(t *testing.T) {
		apple := Fruit{
			ID:    "1",
			Name:  "Apple",
			Color: ColorGreen,
		}
		banana := Fruit{
			ID:    "2",
			Name:  "Banana",
			Color: ColorYellow,
		}

		apple2 := Fruit{}
		banana2 := Fruit{}

		store := newStore()
		err := store.Create(apple, banana)
		require.Nil(t, err, "unexpected error putting records")

		err = store.Read("1", &apple2)
		require.Nil(t, err, "unexpected error getting apple")
		assert.Equal(t, apple, apple2)

		err = store.Read("2", &banana2)
		require.Nil(t, err, "unexpected error getting banana")
		assert.Equal(t, banana, banana2)
	})

	t.Run("TestCreateConflict", func(t *testing.T) {
		apple := Fruit{
			ID:    "1",
			Name:  "Apple",
			Color: ColorGreen,
		}
		apple2 := Fruit{
			ID:    "1",
			Name:  "Apple",
			Color: ColorRed,
		}

		store := newStore()
		err := store.Create(apple)
		require.Nil(t, err, "unexpected error putting records")

		err = store.Create(apple2)
		assert.ErrorIs(t, err, storage.ErrAlreadyExists, "expected conflict error")
	})

	t.Run("TestCreateBadModel", func(t *testing.T) {
		bm := BadModel{ID: "XXX"}
		bm.Cycle = &bm

		store := newStore()
		err := store.Create(bm)
		assert.ErrorIs(t, err, storage.ErrInvalidModel, "expected invalid model error")
	})

	t.Run("TestReadNotFound", func(t *testing.T) {
		store := newStore()
		err := store.Read("1", &Fruit{})
		assert.ErrorIs(t, err, storage.ErrNotFound)

		err = store.Create(&Fruit{ID: "1", Name: "Apple"})
		require.Nil(t, err, "unexpected error creating records")

		err = store.Read("2", &Fruit{})
		assert.ErrorIs(t, err, storage.ErrNotFound)
	})

	t.Run("TestReadWithNilPointer", func(t *testing.T) {
		apple := Fruit{
			ID:    "1",
			Name:  "Apple",
			Color: ColorGreen,
		}

		var apple2 *Fruit

		store := newStore()
		err := store.Create(apple)
		require.Nil(t, err, "unexpected error putting records")

		err = store.Read("1", apple2)
		assert.ErrorIs(t, err, storage.ErrNilModel, "expected nil model error")
	})

	t.Run("TestUpdate", func(t *testing.T) {
		apple := Fruit{
			ID:    "1",
			Name:  "Apple",
			Color: ColorGreen,
		}

		apple2 := Fruit{}

		store := newStore()
		err := store.Create(apple)
		require.Nil(t, err, "unexpected error putting records")

		err = store.Read("1", &apple2)
		require.Nil(t, err, "unexpected error getting apple")
		assert.Equal(t, apple, apple2)

		apple.Color = ColorRed
		err = store.Update(apple)
		require.Nil(t, err, "unexpected error updating apple")

		err = store.Read("1", &apple2)
		require.Nil(t, err, "unexpected error getting apple")
		assert.Equal(t, apple, apple2)
	})

	t.Run("TestUpdateNotExists", func(t *testing.T) {
		apple := Fruit{
			ID:    "1",
			Name:  "Apple",
			Color: ColorGreen,
		}

		store := newStore()
		err := store.Update(apple)
		assert.ErrorIs(t, err, storage.ErrNotFound, "expected not found error")
	})

	t.Run("TestUpdateBadModel", func(t *testing.T) {
		bm := BadModel{ID: "XXX"}
		bm.Cycle = &bm

		store := newStore()
		err := store.Update(bm)
		assert.ErrorIs(t, err, storage.ErrInvalidModel, "expected invalid model error")
	})

	t.Run("TestUpsert", func(t *testing.T) {
		apple := Fruit{
			ID:    "1",
			Name:  "Apple",
			Color: ColorGreen,
		}

		apple2 := Fruit{}
		banana2 := Fruit{}

		store := newStore()
		err := store.Create(apple)
		require.Nil(t, err, "unexpected error putting records")

		apple.Color = ColorRed
		banana := Fruit{ID: "2", Name: "Banana", Color: ColorYellow}
		err = store.Upsert(apple, banana)
		require.Nil(t, err, "unexpected error updating apple")

		err = store.Read("1", &apple2)
		require.Nil(t, err, "unexpected error getting apple")
		assert.Equal(t, apple, apple2)

		err = store.Read("2", &banana2)
		require.Nil(t, err, "unexpected error getting banana")
		assert.Equal(t, banana, banana2)
	})

	t.Run("TestUpsertBadModel", func(t *testing.T) {
		bm := BadModel{ID: "XXX"}
		bm.Cycle = &bm

		store := newStore()
		err := store.Upsert(bm)
		assert.ErrorIs(t, err, storage.ErrInvalidModel, "expected invalid model error")
	})

	t.Run("TestDelete", func(t *testing.T) {
		store := newStore()
		err := store.Create(&Fruit{ID: "4", Name: "Mellon"})
		assert.Nil(t, err)

		exists, err := store.Exists("4", &Fruit{})
		assert.True(t, exists)
		assert.Nil(t, err)

		err = store.Delete(&Fruit{ID: "4"})
		assert.Nil(t, err)

		exists, err = store.Exists("4", &Fruit{})
		assert.False(t, exists)
		assert.Nil(t, err)

		err = store.Delete(&Fruit{ID: "4"})
		assert.ErrorIs(t, err, storage.ErrNotFound, "expected not found error")
	})

	t.Run("TestListErrorCases", func(t *testing.T) {
		store := newStore()

		out := []Fruit{}

		tests := []struct {
			name    string
			models  any
			filter  storage.Model
			wantErr error
		}{
			{"Ok", &out, Fruit{}, nil},
			{"Not a slice", Fruit{}, Fruit{}, storage.ErrSliceRequired},
			{"Not a pointer", out, Fruit{}, storage.ErrSliceRequired},
			{"Mismatched type", &out, Planet{}, storage.ErrTypeMismatch},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if err := store.List(tt.models, tt.filter); err != tt.wantErr {
					t.Errorf("store.List() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("TestList", func(t *testing.T) {

		store := newStore()
		err := store.Create(
			Fruit{"1", "Apple", ColorGreen, nil},
			Fruit{"2", "Banana", ColorYellow, nil},
			Fruit{"3", "Mango", ColorOrange, nil},
		)
		assert.Nil(t, err)

		actual := []Fruit{}
		err = store.List(&actual, Fruit{})
		assert.Nil(t, err)

		expected := []Fruit{
			{"1", "Apple", ColorGreen, nil},
			{"2", "Banana", ColorYellow, nil},
			{"3", "Mango", ColorOrange, nil},
		}

		assert.Equal(t, expected, actual)
	})

	t.Run("TestListFilter", func(t *testing.T) {

		store := newStore()
		err := store.Create(
			Fruit{"1", "Apple", ColorGreen, nil},
			Fruit{"2", "Banana", ColorYellow, nil},
			Fruit{"3", "Mango", ColorOrange, nil},
			Fruit{"4", "Cherry", ColorRed, nil},
			Fruit{"5", "Grape", ColorGreen, nil},
			Fruit{"6", "Strawberry", ColorRed, nil},
			Fruit{"7", "Plum", ColorPurple, nil},
			Fruit{"8", "Tomato", ColorRed, nil},
		)
		assert.Nil(t, err)

		actual := []Fruit{}
		err = store.List(&actual, Fruit{Color: ColorGreen})
		assert.Nil(t, err)

		expected := []Fruit{
			{"1", "Apple", ColorGreen, nil},
			{"5", "Grape", ColorGreen, nil},
		}

		assert.Equal(t, expected, actual)
	})

	t.Run("TestListFilterZero", func(t *testing.T) {

		store := newStore()
		err := store.Create(
			Fruit{"1", "Apple", ColorGreen, pint(4)},
			Fruit{"2", "Banana", ColorYellow, pint(3)},
			Fruit{"3", "Mango", ColorOrange, pint(0)},
			Fruit{"4", "Cherry", ColorRed, pint(0)},
			Fruit{"5", "Grape", ColorGreen, nil},
		)
		assert.Nil(t, err)

		actual := []Fruit{}
		err = store.List(&actual, Fruit{Count: pint(0)})
		assert.Nil(t, err)

		expected := []Fruit{
			{"3", "Mango", ColorOrange, pint(0)},
			{"4", "Cherry", ColorRed, pint(0)},
		}

		assert.Equal(t, expected, actual)
	})

	t.Run("TestExists", func(t *testing.T) {
		store := newStore()
		exists, err := store.Exists("3", &Fruit{})
		assert.False(t, exists)
		assert.Nil(t, err)

		err = store.Create(&Fruit{ID: "3", Name: "Mango"})
		assert.Nil(t, err)

		exists, err = store.Exists("3", &Fruit{})
		assert.True(t, exists)
		assert.Nil(t, err)
	})
}
