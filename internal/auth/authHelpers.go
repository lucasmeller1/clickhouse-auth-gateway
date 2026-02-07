package auth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lucasmeller1/excel_api/internal/config"

	"strings"
)

type ClaimsEntraID struct {
	Groups   []string `json:"groups"`
	TenantID string   `json:"tid"`
	Version  string   `json:"ver"`
	jwt.RegisteredClaims
}

type EntraIDKey struct {
	Kty           string   `json:"kty"`
	Use           string   `json:"use"`
	Kid           string   `json:"kid"`
	X5t           string   `json:"x5t"`
	N             string   `json:"n"`
	E             string   `json:"e"`
	X5c           []string `json:"x5c"`
	CloudInstance string   `json:"cloud_instance_name"`
	Issuer        string   `json:"issuer"`
}

type EntraIDResponse struct {
	Keys []EntraIDKey `json:"keys"`
}

type contextKey int

const ClaimsContextKey contextKey = iota

func ClaimsFromContext(ctx context.Context) (*ClaimsEntraID, bool) {
	claims, ok := ctx.Value(ClaimsContextKey).(*ClaimsEntraID)
	return claims, ok
}

func validateEntraIDKey(key EntraIDKey) bool {
	if key.Kty != "RSA" {
		return false
	}

	if key.Use != "sig" {
		return false
	}

	if key.Kid == "" {
		return false
	}

	if key.N == "" || key.E == "" {
		return false
	}

	return true
}

func searchEntraIDKey(kid string, keys EntraIDResponse) (EntraIDKey, error) {
	for _, key := range keys.Keys {
		if key.Kid == kid {
			return key, nil
		}
	}
	return EntraIDKey{}, fmt.Errorf("key not found in EntraID response")
}

func GetBearerToken(headers http.Header) (string, error) {
	authorization := headers.Get("Authorization")
	if authorization == "" {
		return "", fmt.Errorf("authorization header missing")
	}

	const prefix = "Bearer "
	if (len(authorization) < len(prefix)) || !strings.EqualFold(prefix, authorization[:len(prefix)]) {
		return "", fmt.Errorf("authorization scheme is not a bearer")
	}

	token := strings.TrimSpace(authorization[len(prefix):])
	if token == "" {
		return "", fmt.Errorf("token is missing")
	}

	return token, nil
}

func validateClaims(claims *ClaimsEntraID, cfg config.AuthConfig) error {
	if claims.TenantID != cfg.TenantID {
		return fmt.Errorf("tenant mismatch")
	}
	return nil
}

func finalizeClaims(token *jwt.Token, cfg config.AuthConfig) (*ClaimsEntraID, error) {

	claims, ok := token.Claims.(*ClaimsEntraID)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}

	if err := validateClaims(claims, cfg); err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	return claims, nil
}

func isSignatureError(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()

	return strings.Contains(msg, "signature") ||
		strings.Contains(msg, "verification error") ||
		strings.Contains(msg, "crypto/rsa")
}
