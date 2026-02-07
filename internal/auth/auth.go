package auth

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/lucasmeller1/excel_api/internal/handlers"
	"github.com/lucasmeller1/excel_api/internal/redis"

	//"os"
	"strings"
	"time"
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

func GetEntraIDPublicKey(ctx context.Context, cfgAuth *config.AuthConfig, redisClient *redis.RedisClient, kid string) (EntraIDKey, error) {
	cachedBytes, err := redisClient.GetWithSingleflight(ctx, fmt.Sprintf("jwks:%s", cfgAuth.TenantID), time.Hour, func() ([]byte, error) {
		url := fmt.Sprintf("https://login.microsoftonline.com/%s/discovery/v2.0/keys", cfgAuth.TenantID)

		dataBytes, err := handlers.GetRequest(ctx, url)
		if err != nil {
			return nil, err
		}

		return dataBytes, nil
	})

	if err != nil {
		return EntraIDKey{}, fmt.Errorf("failed to get tid keys: %w", err)
	}

	var keys EntraIDResponse
	err = json.Unmarshal(cachedBytes, &keys)
	if err != nil {
		return EntraIDKey{}, fmt.Errorf("failed to unmarshal EntraID response: %w", err)
	}

	key, err := searchEntraIDKey(kid, keys)
	if err != nil {
		return EntraIDKey{}, err
	}

	if !validateEntraIDKey(key) {
		return EntraIDKey{}, fmt.Errorf("invalid JWKS key")
	}

	return key, nil
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

func IsValidJWTEntra(ctx context.Context, jwtToken string, cfg config.AuthConfig, redisClient *redis.RedisClient) (*ClaimsEntraID, error) {
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
		&ClaimsEntraID{},
		func(token *jwt.Token) (any, error) {
			_, ok := token.Method.(*jwt.SigningMethodRSA)
			if !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}

			kid, ok := token.Header["kid"].(string)
			if !ok || kid == "" {
				return nil, fmt.Errorf("missing kid in token header")
			}

			entraKey, err := GetEntraIDPublicKey(ctx, &cfg, redisClient, kid)
			if err != nil {
				return nil, err
			}

			publicKey, err := rsaPublicKeyFromEntraJWK(entraKey)
			if err != nil {
				return nil, err
			}

			return publicKey, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("jwt validation failed: %w", err)
	}

	claims, ok := token.Claims.(*ClaimsEntraID)
	if !ok {
		return nil, fmt.Errorf("invalid claims type")
	}

	err = validateClaims(claims, cfg)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	return claims, nil
}

func rsaPublicKeyFromEntraJWK(key EntraIDKey) (*rsa.PublicKey, error) {
	raw, err := json.Marshal(map[string]any{
		"kty": key.Kty,
		"kid": key.Kid,
		"use": key.Use,
		"n":   key.N,
		"e":   key.E,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JWK: %w", err)
	}

	jwkKey, err := jwk.ParseKey(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWK: %w", err)
	}

	var pubKey rsa.PublicKey
	if err := jwkKey.Raw(&pubKey); err != nil {
		return nil, fmt.Errorf("failed to extract RSA public key: %w", err)
	}

	return &pubKey, nil
}
