package auth

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lucasmeller1/clickhouse-auth-gateway/internal/config"
	"github.com/lucasmeller1/clickhouse-auth-gateway/internal/handlers"
	"github.com/lucasmeller1/clickhouse-auth-gateway/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"strings"
)

type ClaimsEntraID struct {
	Groups   []string `json:"groups"`
	TenantID string   `json:"tid"`
	Version  string   `json:"ver"`
	OID      string   `json:"oid"`
	jwt.RegisteredClaims
}

type openIDConfig struct {
	JWKSURI string `json:"jwks_uri"`
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

func GetUserOID(ctx context.Context) (string, error) {
	claims, ok := ClaimsFromContext(ctx)

	if !ok {
		return "", fmt.Errorf("claims not found in context")
	}

	if claims == nil {
		return "", fmt.Errorf("claims found in context but object is nil")
	}

	if claims.OID == "" {
		return "", fmt.Errorf("user authenticated but OID is missing from claims")
	}

	return claims.OID, nil
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

	if token == nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

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

	return errors.Is(err, jwt.ErrTokenSignatureInvalid)
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

func FetchEntraJWKS(ctx context.Context, cfgAuth *config.AuthConfig) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	ctx, span := tracer.Start(ctx, "Auth.FetchEntraJWKS", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	start := time.Now()
	defer func() {
		span.SetAttributes(
			attribute.Float64("http.duration_ms",
				float64(time.Since(start).Milliseconds())),
		)
	}()

	// 1. Fetch OpenID Configuration
	openIDURL := fmt.Sprintf(
		"https://login.microsoftonline.com/%s/v2.0/.well-known/openid-configuration",
		cfgAuth.TenantID,
	)

	span.SetAttributes(
		attribute.String("tenant.id", cfgAuth.TenantID),
		attribute.String("openid.config.url", openIDURL),
	)

	configBytes, err := handlers.GetRequest(ctx, openIDURL)
	if err != nil {
		telemetry.RecordSpanError(span, err)
		return nil, fmt.Errorf("failed to fetch openid configuration: %w", err)
	}

	var oidc openIDConfig
	if err := json.Unmarshal(configBytes, &oidc); err != nil {
		telemetry.RecordSpanError(span, err)
		return nil, fmt.Errorf("invalid openid configuration response: %w", err)
	}

	if oidc.JWKSURI == "" {
		err := errors.New("jwks_uri missing from openid configuration")
		telemetry.RecordSpanError(span, err)
		return nil, err
	}

	span.SetAttributes(
		attribute.String("jwks.uri", oidc.JWKSURI),
	)

	// 2. Fetch JWKS
	dataBytes, err := handlers.GetRequest(ctx, oidc.JWKSURI)
	if err != nil {
		telemetry.RecordSpanError(span, err)
		return nil, fmt.Errorf("failed to fetch jwks: %w", err)
	}

	span.SetAttributes(
		attribute.Int("http.response_size_bytes", len(dataBytes)),
	)

	return dataBytes, nil
}
