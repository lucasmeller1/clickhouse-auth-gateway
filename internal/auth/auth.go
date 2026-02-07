package auth

import (
	"context"
	"encoding/json"
	//"crypto/rsa"
	"fmt"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/lucasmeller1/excel_api/internal/handlers"
	"github.com/lucasmeller1/excel_api/internal/redis"

	//"os"
	"strings"
	"time"
)

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

func GetEntraPublicKey(ctx context.Context, cfgAuth *config.AuthConfig, redisClient *redis.RedisClient, kid string) (EntraIDKey, error) {

	cachedBytes, err := redisClient.GetWithSingleflight(ctx, fmt.Sprintf("jwks:%s", kid), time.Hour, func() ([]byte, error) {
		url := fmt.Sprintf("https://login.microsoftonline.com/%s/discovery/v2.0/keys", cfgAuth.TenantID)

		dataBytes, err := handlers.GetRequest(ctx, url)
		if err != nil {
			return nil, err
		}

		var keys EntraIDResponse
		err = json.Unmarshal(dataBytes, &keys)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal EntraID response: %w", err)
		}

		key, err := searchKey(kid, keys)
		if err != nil {
			return nil, err
		}

		return json.Marshal(key)
	})

	if err != nil {
		return EntraIDKey{}, fmt.Errorf("failed to get key: %w", err)
	}

	var result EntraIDKey
	err = json.Unmarshal(cachedBytes, &result)
	if err != nil {
		return EntraIDKey{}, fmt.Errorf("failed to decode cached key: %w", err)
	}

	return result, nil

}

func searchKey(kid string, keys EntraIDResponse) (EntraIDKey, error) {
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

func validateClaims(claims *CustomClaims, cfg config.AuthConfig) error {
	if claims.TenantID != cfg.TenantID {
		return fmt.Errorf("tenant mismatch")
	}
	return nil
}

func IsValidJWTEntra(ctx context.Context, jwtToken string, cfg config.AuthConfig) (*CustomClaims, error) {
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
