package prefab

import (
	"path/filepath"
	"testing"
)

func TestSearchForConfig_defaultConfig(t *testing.T) {
	actual := searchForConfig("prefab.yaml", "./templates/default")
	expected, err := filepath.Abs("./prefab.yaml")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	if actual != expected {
		t.Fatalf("Expected %s, got %s", expected, actual)
	}
}

func TestSearchForConfig_noConfig(t *testing.T) {
	actual := searchForConfig("prefab-rando-11234.yaml", "./templates/default")
	if actual != "" {
		t.Fatalf("Expected empty string, got %s", actual)
	}
}

func TestTransformEnv(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "PF__SERVER__INCOMING_HEADERS", want: "server.incomingHeaders"},
		{input: "PF__FOOBAR", want: "foobar"},
		{input: "PF__A__B_C", want: "a.bC"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := transformEnv(tt.input); got != tt.want {
				t.Errorf("transformEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}
