package token

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"gitlab.com/syncbox/backend/internal/domain"
)

// TokenKind distingue access tokens (cortos) de refresh tokens (largos).
type TokenKind string

const (
	KindAccess  TokenKind = "access"
	KindRefresh TokenKind = "refresh"
)

// Claims son los claims propios de Syncbox empaquetados en el JWT.
type Claims struct {
	UserID uuid.UUID `json:"uid"`
	Role   string    `json:"rol"`
	Kind   TokenKind `json:"knd"`
	JTI    string    `json:"jti"`
	jwt.RegisteredClaims
}

// Manager firma y verifica tokens HS256 con un secreto compartido.
type Manager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	issuer     string
	audience   string
}

// NewManager construye un Manager con la configuración indicada.
func NewManager(secret []byte, accessTTL, refreshTTL time.Duration, issuer, audience string) *Manager {
	return &Manager{
		secret:     secret,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		issuer:     issuer,
		audience:   audience,
	}
}

// Issue emite un token firmado del tipo solicitado y devuelve también el jti
// generado, que el caller debe persistir en `sesion`.
func (m *Manager) Issue(userID uuid.UUID, role domain.Role, kind TokenKind) (string, string, time.Time, error) {
	jti := uuid.NewString()
	now := time.Now().UTC()
	ttl := m.accessTTL
	if kind == KindRefresh {
		ttl = m.refreshTTL
	}
	exp := now.Add(ttl)

	claims := Claims{
		UserID: userID,
		Role:   string(role),
		Kind:   kind,
		JTI:    jti,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Audience:  jwt.ClaimStrings{m.audience},
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-30 * time.Second)),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        jti,
		},
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString(m.secret)
	if err != nil {
		return "", "", time.Time{}, err
	}
	return signed, jti, exp, nil
}

// Parse valida la firma y los claims estándar del token y devuelve los claims.
// No verifica revocación: eso lo hace el middleware contra `sesion`.
func (m *Manager) Parse(raw string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(raw, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("método de firma inesperado")
		}
		return m.secret, nil
	},
		jwt.WithIssuer(m.issuer),
		jwt.WithAudience(m.audience),
		jwt.WithLeeway(60*time.Second),
		jwt.WithValidMethods([]string{"HS256"}),
	)
	if err != nil {
		return nil, domain.ErrTokenInvalid
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, domain.ErrTokenInvalid
	}
	return claims, nil
}
