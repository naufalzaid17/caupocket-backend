// Package password menyediakan helper hashing password berbasis bcrypt.
package password

import "golang.org/x/crypto/bcrypt"

const bcryptCost = 12

// HashPassword menghasilkan bcrypt hash dari password plain text.
func HashPassword(plain string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// CheckPasswordHash membandingkan password plain text dengan bcrypt hash tersimpan.
func CheckPasswordHash(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
