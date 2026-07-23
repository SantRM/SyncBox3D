package crypto

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// Coste razonable para 2026: ~250 ms por hash en hardware moderno.
const bcryptCost = 12

// MaxPasswordBytes es el límite duro impuesto por bcrypt: cualquier byte
// después del 72º es ignorado silenciosamente. Validamos explícitamente
// para evitar que el usuario crea tener una contraseña más larga de la
// que realmente protege su cuenta.
const MaxPasswordBytes = 72

// ErrPasswordTooLong indica que la contraseña excede el límite de bcrypt.
var ErrPasswordTooLong = errors.New("contrase\u00f1a excede 72 bytes")

// HashPassword genera un bcrypt-hash de la contraseña en texto plano.
func HashPassword(plain string) (string, error) {
	if len(plain) > MaxPasswordBytes {
		return "", ErrPasswordTooLong
	}
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// VerifyPassword compara un hash bcrypt contra la contraseña en texto plano.
// Devuelve nil si coinciden. La librería ya hace comparación de tiempo constante.
// Se trunca explícitamente a 72 bytes para que el comportamiento de verificación
// sea estable y consistente con HashPassword (que rechaza >72 al crear, pero
// hashes legados podrían haberse generado con valores más cortos efectivos).
func VerifyPassword(hash, plain string) error {
	if len(plain) > MaxPasswordBytes {
		plain = plain[:MaxPasswordBytes]
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}
