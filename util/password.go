package util

import (
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func IsHashedPassword(stored string) bool {
	return strings.HasPrefix(stored, "$2")
}

func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func CheckPassword(plain, stored string) bool {
	if IsHashedPassword(stored) {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(plain)) == nil
	}
	return plain == stored
}
