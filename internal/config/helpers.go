package config

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// SearchForConfig recursively searches for a config file starting from startDir
// and walking up the directory tree until found or reaching the root.
func SearchForConfig(filename string, startDir string) string {
	d, err := filepath.Abs(startDir)
	if err != nil {
		return ""
	}

	p := filepath.Join(d, filename)
	if _, err = os.Stat(p); err == nil {
		return p
	}

	parentDir := filepath.Dir(d)
	if parentDir == d {
		return ""
	}
	return SearchForConfig(filename, parentDir)
}

// TransformEnv converts PF__SERVER__INCOMING_HEADERS to server.incomingHeaders.
// Environment variable transformation rules:
//   - Remove PF__ prefix
//   - Convert to lowercase
//   - Double underscores (__) become dots (.)
//   - Single underscores (_) within segments become camelCase
func TransformEnv(s string) string {
	s = strings.ToLower(strings.TrimPrefix(s, "PF__"))
	segments := strings.Split(s, "__")
	for i, segment := range segments {
		parts := strings.Split(segment, "_")
		for j := 1; j < len(parts); j++ {
			parts[j] = capitalize(parts[j])
		}
		segments[i] = strings.Join(parts, "")
	}

	return strings.Join(segments, ".")
}

// capitalize capitalizes the first rune of a string.
func capitalize(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
