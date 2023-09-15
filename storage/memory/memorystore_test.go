package memory

import (
	"testing"

	"github.com/dpup/prefab/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Color int

const (
	ColorRed     Color = 1
	ColorGreen   Color = 2
	ColorOrgange Color = 3
	ColorYellow  Color = 4
	ColorBlue    Color = 5
	ColorPurple  Color = 6
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

func pint(i int) *int {
	return &i
}

func TestSaveGetRoundTrip(t *testing.T) {
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

	store := New()
	err := store.Put(apple, banana)
	require.Nil(t, err, "unexpected error putting records")

	err = store.Get("1", &apple2)
	require.Nil(t, err, "unexpected error getting apple")
	assert.Equal(t, apple, apple2)

	err = store.Get("2", &banana2)
	require.Nil(t, err, "unexpected error getting banana")
	assert.Equal(t, banana, banana2)
}

func TestExists(t *testing.T) {
	store := New()
	exists, err := store.Exists("3", &Fruit{})
	assert.False(t, exists)
	assert.Nil(t, err)

	err = store.Put(&Fruit{ID: "3", Name: "Mango"})
	assert.Nil(t, err)

	exists, err = store.Exists("3", &Fruit{})
	assert.True(t, exists)
	assert.Nil(t, err)
}

func TestDelete(t *testing.T) {
	store := New()
	err := store.Put(&Fruit{ID: "4", Name: "Mellon"})
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
	assert.Equal(t, storage.ErrNotFound, err)
}

func TestListErrorCases(t *testing.T) {
	store := New()

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
}

func TestList(t *testing.T) {

	store := New()
	err := store.Put(
		Fruit{"1", "Apple", ColorGreen, nil},
		Fruit{"2", "Banana", ColorYellow, nil},
		Fruit{"3", "Mango", ColorOrgange, nil},
	)
	assert.Nil(t, err)

	actual := []Fruit{}
	err = store.List(&actual, Fruit{})
	assert.Nil(t, err)

	expected := []Fruit{
		{"1", "Apple", ColorGreen, nil},
		{"2", "Banana", ColorYellow, nil},
		{"3", "Mango", ColorOrgange, nil},
	}

	assert.Equal(t, expected, actual)
}

func TestListFilter(t *testing.T) {

	store := New()
	err := store.Put(
		Fruit{"1", "Apple", ColorGreen, nil},
		Fruit{"2", "Banana", ColorYellow, nil},
		Fruit{"3", "Mango", ColorOrgange, nil},
		Fruit{"4", "Cherry", ColorRed, nil},
		Fruit{"5", "Grape", ColorGreen, nil},
		Fruit{"6", "Stawberry", ColorRed, nil},
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
}

func TestListFilterZero(t *testing.T) {

	store := New()
	err := store.Put(
		Fruit{"1", "Apple", ColorGreen, pint(4)},
		Fruit{"2", "Banana", ColorYellow, pint(3)},
		Fruit{"3", "Mango", ColorOrgange, pint(0)},
		Fruit{"4", "Cherry", ColorRed, pint(0)},
		Fruit{"5", "Grape", ColorGreen, nil},
	)
	assert.Nil(t, err)

	actual := []Fruit{}
	err = store.List(&actual, Fruit{Count: pint(0)})
	assert.Nil(t, err)

	expected := []Fruit{
		{"3", "Mango", ColorOrgange, pint(0)},
		{"4", "Cherry", ColorRed, pint(0)},
	}

	assert.Equal(t, expected, actual)
}
