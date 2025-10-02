package pwdauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestBcryptHasher_Generate(t *testing.T) {
	hasher := bcryptHasher{}
	password := []byte("my-secure-password")

	hashed, err := hasher.Generate(password)
	require.NoError(t, err)
	assert.NotEmpty(t, hashed)
	assert.NotEqual(t, password, hashed)

	// Simply verify it's a valid bcrypt hash
	err = bcrypt.CompareHashAndPassword(hashed, password)
	assert.NoError(t, err)
}

func TestBcryptHasher_Compare(t *testing.T) {
	hasher := bcryptHasher{}
	password := []byte("my-secure-password")

	// Generate a hash
	hashed, err := hasher.Generate(password)
	require.NoError(t, err)

	tests := []struct {
		name          string
		hashedPwd     []byte
		plainPwd      []byte
		expectedError bool
	}{
		{
			name:          "correct password",
			hashedPwd:     hashed,
			plainPwd:      password,
			expectedError: false,
		},
		{
			name:          "incorrect password",
			hashedPwd:     hashed,
			plainPwd:      []byte("wrong-password"),
			expectedError: true,
		},
		{
			name:          "empty password",
			hashedPwd:     hashed,
			plainPwd:      []byte(""),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := hasher.Compare(tt.hashedPwd, tt.plainPwd)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTestHasher_Generate(t *testing.T) {
	hasher := testHasher{}
	password := []byte("test-password")

	hashed, err := hasher.Generate(password)
	require.NoError(t, err)
	assert.Equal(t, password, hashed, "test hasher should return password as-is")
}

func TestTestHasher_Compare(t *testing.T) {
	hasher := testHasher{}

	tests := []struct {
		name          string
		hashedPwd     []byte
		plainPwd      []byte
		expectedError bool
	}{
		{
			name:          "matching passwords",
			hashedPwd:     []byte("password123"),
			plainPwd:      []byte("password123"),
			expectedError: false,
		},
		{
			name:          "non-matching passwords",
			hashedPwd:     []byte("password123"),
			plainPwd:      []byte("different"),
			expectedError: true,
		},
		{
			name:          "empty passwords",
			hashedPwd:     []byte(""),
			plainPwd:      []byte(""),
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := hasher.Compare(tt.hashedPwd, tt.plainPwd)
			if tt.expectedError {
				require.Error(t, err)
				assert.Equal(t, bcrypt.ErrMismatchedHashAndPassword, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultHasher(t *testing.T) {
	// Verify DefaultHasher is bcryptHasher
	password := []byte("test-password")

	hashed, err := DefaultHasher.Generate(password)
	require.NoError(t, err)
	assert.NotEmpty(t, hashed)

	err = DefaultHasher.Compare(hashed, password)
	assert.NoError(t, err)
}

func TestTestHasher_Exported(t *testing.T) {
	// Verify TestHasher is accessible and functional
	password := []byte("test-password")

	hashed, err := TestHasher.Generate(password)
	require.NoError(t, err)
	assert.Equal(t, password, hashed)

	err = TestHasher.Compare(hashed, password)
	assert.NoError(t, err)
}
