package storage

import "testing"

type Fruit struct {
	ID   string
	Name string
}

func (f Fruit) PK() string {
	return f.ID
}

type CapitalCity struct {
	ID   string
	Name string
}

func (c CapitalCity) PK() string {
	return c.ID
}

type Vehicle struct {
	ID     string
	Wheels int
}

func (v Vehicle) PK() string {
	return v.ID
}

func (v Vehicle) Name() string {
	return "cars"
}

func TestName(t *testing.T) {
	tests := []struct {
		name  string
		model any
		want  string
	}{
		{name: "single word struct", model: Fruit{}, want: "fruits"},
		{name: "multi word struct", model: CapitalCity{}, want: "capital_cities"},
		{name: "manual override", model: Vehicle{}, want: "cars"},
		{name: "slice", model: []Fruit{}, want: "fruits"},
	}
	for i := 0; i < 3; i++ {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := Name(tt.model); got != tt.want {
					t.Errorf("Iter %d. Name() = %v, want %v", i, got, tt.want)
				}
			})
		}
	}
}
