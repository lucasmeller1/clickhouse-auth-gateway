package auth

import (
	"crypto/rsa"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lucasmeller1/excel_api/internal/config"
	"os"
	"time"
)

const (
	privateKeyPath = "./private.pem"
	publicKeyPath  = "./public.pem"
)

var schemaToGroup = map[string]string{
	"Contabil_1": "11111111-1111-1111-1111-111111111111",
	"Contabil_2": "11111111-1111-1111-1111-111111111112",
	"Contabil_3": "11111111-1111-1111-1111-111111111113",
	"Contabil_4": "11111111-1111-1111-1111-111111111114",
	"Contabil_5": "11111111-1111-1111-1111-111111111115",
	"Contabil_6": "11111111-1111-1111-1111-111111111116",
	"Contabil_7": "11111111-1111-1111-1111-111111111117",

	"Financeiro_1": "22222222-2222-2222-2222-222222222221",
	"Financeiro_2": "22222222-2222-2222-2222-222222222222",
	"Financeiro_3": "22222222-2222-2222-2222-222222222223",
	"Financeiro_4": "22222222-2222-2222-2222-222222222224",
	"Financeiro_5": "22222222-2222-2222-2222-222222222225",
	"Financeiro_6": "22222222-2222-2222-2222-222222222226",
	"Financeiro_7": "22222222-2222-2222-2222-222222222227",

	"Operacional_1": "33333333-3333-3333-3333-333333333331",
	"Operacional_2": "33333333-3333-3333-3333-333333333332",
	"Operacional_3": "33333333-3333-3333-3333-333333333333",
	"Operacional_4": "33333333-3333-3333-3333-333333333334",
	"Operacional_5": "33333333-3333-3333-3333-333333333335",
	"Operacional_6": "33333333-3333-3333-3333-333333333336",
	"Operacional_7": "33333333-3333-3333-3333-333333333337",
}

type CustomClaims struct {
	Groups   []string `json:"groups"`
	TenantID string   `json:"tid"`
	jwt.RegisteredClaims
}

// ========= para teste local
func CreateSignedToken(cfg config.AuthConfig, groups []string) (string, error) {
	privateKeyPEM, err := loadPrivateKey(privateKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	guidGroups := make([]string, 0)
	for _, g := range groups {
		guidGroups = append(guidGroups, schemaToGroup[g])
	}

	userClaims := CustomClaims{
		Groups:   guidGroups,
		TenantID: cfg.TenantID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "local_excel_api",
			Subject:   "lucasmeller",
			Audience:  []string{"excel_api"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Second * 30)),
			NotBefore: jwt.NewNumericDate(time.Now()),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, userClaims)
	token.Header["kid"] = cfg.KeyID

	signedToken, err := token.SignedString(privateKeyPEM)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signedToken, nil
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}
	return jwt.ParseRSAPrivateKeyFromPEM(key)
}

func loadPublicKey(path string) (*rsa.PublicKey, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}
	return jwt.ParseRSAPublicKeyFromPEM(key)
}

// =========

func validateClaims(claims *CustomClaims, cfg config.AuthConfig) error {
	if claims.TenantID != cfg.TenantID {
		return fmt.Errorf("tenant mismatch")
	}
	return nil
}

func IsValidJWT(jwtToken string, cfg config.AuthConfig) (*CustomClaims, error) {
	publicKey, err := loadPublicKey(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load public key: %w", err)
	}

	parser := jwt.NewParser(
		jwt.WithIssuer(cfg.Issuer),
		jwt.WithAudience(cfg.Audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(2*time.Minute),
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
	)

	token, err := parser.ParseWithClaims(
		jwtToken,
		&CustomClaims{},
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return publicKey, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("jwt validation failed: %w", err)
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}

	err = validateClaims(claims, cfg)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	return claims, nil
}
