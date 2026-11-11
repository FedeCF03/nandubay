// Package seguridad implementa los puertos Hasher y Tokens de la aplicación.
package seguridad

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

// Bcrypt guarda contraseñas con bcrypt.
type Bcrypt struct {
	Costo int // 0 = 12
	once  sync.Once
	falso []byte
}

func (b *Bcrypt) costo() int {
	if b.Costo == 0 {
		return 12
	}
	return b.Costo
}

func (b *Bcrypt) Hash(clave string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(clave), b.costo())
	return string(h), err
}

// Comparar con hash vacío compara contra un hash falso, para que tarde lo mismo.
func (b *Bcrypt) Comparar(hash, clave string) bool {
	if hash == "" {
		b.once.Do(func() {
			b.falso, _ = bcrypt.GenerateFromPassword([]byte("no-existe-este-usuario"), b.costo())
		})
		_ = bcrypt.CompareHashAndPassword(b.falso, []byte(clave))
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(clave)) == nil
}

// Tokens genera tokens aleatorios de 256 bits y los hashea con SHA-256.
type Tokens struct{}

func (Tokens) Nuevo() (string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	t := base64.RawURLEncoding.EncodeToString(b)
	return t, Tokens{}.Hash(t), nil
}

func (Tokens) Hash(token string) string {
	s := sha256.Sum256([]byte(token))
	return hex.EncodeToString(s[:])
}
