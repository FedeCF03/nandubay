package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const minLargoClave = 8

var ErrClaveCorta = errors.New("la contraseña tiene que tener al menos 8 caracteres")

func HashClave(clave string) (string, error) {
	if utf8.RuneCountInString(clave) < minLargoClave {
		return "", ErrClaveCorta
	}
	if len(clave) > 72 {
		return "", errors.New("la contraseña es demasiado larga (máximo 72 caracteres)")
	}
	h, err := bcrypt.GenerateFromPassword([]byte(clave), 12)
	return string(h), err
}

// hashFalso se usa cuando el email no existe, para que la respuesta tarde lo
// mismo y no se pueda adivinar qué emails están registrados.
var hashFalso, _ = bcrypt.GenerateFromPassword([]byte("no-existe-este-usuario"), 12)

func claveCorrecta(hash, clave string) bool {
	if hash == "" {
		_ = bcrypt.CompareHashAndPassword(hashFalso, []byte(clave))
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(clave)) == nil
}

func nuevoToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	s := sha256.Sum256([]byte(token))
	return hex.EncodeToString(s[:])
}
