package pwdauth

// TODO: Update DefaultHasher to use Argon2id
// golang.org/x/crypto/argon2
// https://www.alexedwards.net/blog/how-to-hash-and-verify-passwords-with-argon2-in-go

import "golang.org/x/crypto/bcrypt"

// Interface that allows password hashing to be customized.
type Hasher interface {
	// Generate a hashed password from a plaintext password.
	Generate(password []byte) ([]byte, error)

	// Compare a hashed password with a plaintext password.
	Compare(hashedPassword, password []byte) error
}

// DefaultHasher calls golang's standard bcrypt functions to hash and compare
// passwords.
var DefaultHasher = bcryptHasher{}

// TestHasher is a Hasher that does not hash passwords. It is useful for testing
// purposes.
var TestHasher = testHasher{}

type bcryptHasher struct{}

func (bcryptHasher) Generate(password []byte) ([]byte, error) {
	return bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
}

func (bcryptHasher) Compare(hashedPassword, password []byte) error {
	return bcrypt.CompareHashAndPassword(hashedPassword, password)
}

type testHasher struct{}

func (testHasher) Generate(password []byte) ([]byte, error) {
	return password, nil
}

func (testHasher) Compare(hashedPassword, password []byte) error {
	if string(hashedPassword) != string(password) {
		return bcrypt.ErrMismatchedHashAndPassword
	}
	return nil
}
