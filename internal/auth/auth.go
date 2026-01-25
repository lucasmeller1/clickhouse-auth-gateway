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

type customClaims struct {
	Groups   []string `json:"groups"`
	TenantID string   `json:"tid"`
	KeyID    string   `json:"kid"`
	jwt.RegisteredClaims
}

func CreateSignedToken(cfg config.AuthConfig, groups []string) (string, error) {
	privateKeyPEM, err := loadPrivateKey(privateKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	guidGroups := make([]string, 0)
	for _, g := range groups {
		guidGroups = append(guidGroups, schemaToGroup[g])
	}

	userClaims := customClaims{
		Groups:   guidGroups,
		TenantID: cfg.TenantID,
		KeyID:    cfg.KeyID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "local_excel_api",
			Subject:   "lucasmeller",
			Audience:  []string{"excel_api"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			NotBefore: jwt.NewNumericDate(time.Now()),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, userClaims)
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

// metodo meio estranho, fazer do zero com a documentacao
func validateClaims(claims *customClaims, cfg config.AuthConfig) error {
	if claims.Issuer != cfg.Issuer {
		return fmt.Errorf("invalid issuer")
	}

	/*
		if !claims.VerifyAudience(cfg.Audience, true) {
			return fmt.Errorf("invalid audience")
		}
	*/

	if claims.ExpiresAt == nil || time.Now().After(claims.ExpiresAt.Time) {
		return fmt.Errorf("token expired")
	}

	if claims.NotBefore != nil && time.Now().Before(claims.NotBefore.Time) {
		return fmt.Errorf("token not valid yet")
	}

	if claims.TenantID == "" {
		return fmt.Errorf("missing tenant id")
	}

	return nil
}

func IsValidJWT(jwtToken string, cfg config.AuthConfig) error {
	publicKey, err := loadPublicKey(publicKeyPath)
	if err != nil {
		return fmt.Errorf("failed to load public key: %w", err)
	}

	token, err := jwt.ParseWithClaims(
		jwtToken,
		&customClaims{},
		func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodRS256 {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return publicKey, nil
		},
	)
	if err != nil {
		return fmt.Errorf("failed to check signature method")
	}

	if !token.Valid {
		return fmt.Errorf("invalid token")
	}

	_, ok := token.Claims.(*customClaims)
	if !ok {
		return fmt.Errorf("invalid claims type")
	}

	/*
		err = validateClaims(claims, cfg)
		if err != nil {
			return false, fmt.Errorf("invalid token: %w", err)
		}
	*/

	return nil
}
