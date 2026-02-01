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
		guidGroups = append(guidGroups, config.SchemaToGUID[g])
	}

	userClaims := CustomClaims{
		Groups:   guidGroups,
		TenantID: cfg.TenantID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "local_excel_api",
			Subject:   "lucasmeller",
			Audience:  []string{"excel_api"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 2)),
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
		func(token *jwt.Token) (any, error) {
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
