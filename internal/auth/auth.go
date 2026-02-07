package auth

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/lucasmeller1/excel_api/internal/handlers"
	"github.com/lucasmeller1/excel_api/internal/redis"

	"strings"
	"time"
)

func FetchEntraJWKS(ctx context.Context, cfgAuth *config.AuthConfig) ([]byte, error) {
	url := fmt.Sprintf("https://login.microsoftonline.com/%s/discovery/v2.0/keys", cfgAuth.TenantID)

	dataBytes, err := handlers.GetRequest(ctx, url)
	if err != nil {
		return nil, err
	}

	return dataBytes, nil
}

func GetCachedTIDKeys(ctx context.Context, cfgAuth *config.AuthConfig, redisClient *redis.RedisClient, force bool) ([]byte, error) {

	cacheKey := fmt.Sprintf("jwks:%s", cfgAuth.TenantID)

	if force {
		// bypass cache completely
		data, err := FetchEntraJWKS(ctx, cfgAuth)
		if err != nil {
			return nil, err
		}

		// update cache directly
		_ = redisClient.SetCachedResponse(ctx, cacheKey, data, time.Hour)

		return data, nil
	}

	return redisClient.GetWithSingleflight(ctx, cacheKey, time.Hour, func() ([]byte, error) {
		return FetchEntraJWKS(ctx, cfgAuth)
	})
}

func GetEntraIDPublicKey(ctx context.Context, cfgAuth *config.AuthConfig, redisClient *redis.RedisClient, kid string, force bool) (EntraIDKey, error) {
	cachedBytes, err := GetCachedTIDKeys(ctx, cfgAuth, redisClient, force)
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

func ValidateEntraJWT(ctx context.Context, jwtToken string, cfg config.AuthConfig, redisClient *redis.RedisClient) (*ClaimsEntraID, error) {

	validate := func(force bool) (*jwt.Token, error) {

		parser := jwt.NewParser(
			jwt.WithIssuer(cfg.Issuer),
			jwt.WithAudience(cfg.Audience),
			jwt.WithExpirationRequired(),
			jwt.WithIssuedAt(),
			jwt.WithLeeway(2*time.Minute),
			jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		)

		return parser.ParseWithClaims(
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

				entraKey, err := GetEntraIDPublicKey(ctx, &cfg, redisClient, kid, force)
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
	}

	// FIRST ATTEMPT (using cache)
	token, err := validate(false)

	if err == nil {
		return finalizeClaims(token, cfg)
	}

	retryNeeded :=
		isSignatureError(err) ||
			strings.Contains(err.Error(), "key not found")

	if !retryNeeded {
		return nil, fmt.Errorf("jwt validation failed: %w", err)
	}

	// SECOND ATTEMPT – force refresh JWKS
	token, err = validate(true)
	if err != nil {
		return nil, fmt.Errorf("jwt validation failed after JWKS refresh: %w", err)
	}

	return finalizeClaims(token, cfg)
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
