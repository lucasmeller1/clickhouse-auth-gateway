package auth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lucasmeller1/excel_api/internal/config"
	"github.com/lucasmeller1/excel_api/internal/redis"
	"github.com/lucasmeller1/excel_api/internal/telemetry"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"strings"
	"time"
)

var tracer = otel.Tracer("github.com/lucasmeller1/excel_api/internal/auth")

func GetCachedTIDKeys(ctx context.Context, cfgAuth *config.AuthConfig, redisClient *redis.RedisClient, force bool) ([]byte, error) {
	ctx, span := tracer.Start(ctx, "Auth.GetCachedTIDKeys", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	cacheKey := fmt.Sprintf("jwks:%s", cfgAuth.TenantID)

	span.SetAttributes(
		attribute.String("tenant.id", cfgAuth.TenantID),
		attribute.Bool("jwks.force_refresh", force),
		attribute.String("cache.key", cacheKey),
	)

	if force {
		data, err := FetchEntraJWKS(ctx, cfgAuth)
		if err != nil {
			telemetry.RecordSpanError(span, err)
			return nil, err
		}

		_ = redisClient.SetCachedResponse(ctx, cacheKey, data, time.Hour)

		span.SetAttributes(attribute.String("jwks.cache_status", "force_refresh"))
		return data, nil
	}

	data, err := redisClient.GetWithSingleflight(ctx, cacheKey, time.Hour, func(sfCtx context.Context) ([]byte, error) {
		span.AddEvent("jwks.cache_miss_fetching")
		return FetchEntraJWKS(sfCtx, cfgAuth)
	})

	if err != nil {
		telemetry.RecordSpanError(span, err)
		return nil, err
	}

	return data, nil
}

func GetEntraIDPublicKey(ctx context.Context, cfgAuth *config.AuthConfig, redisClient *redis.RedisClient, kid string, force bool) (EntraIDKey, error) {
	ctx, span := tracer.Start(ctx, "Auth.GetEntraIDPublicKey", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	span.SetAttributes(
		attribute.String("tenant.id", cfgAuth.TenantID),
		attribute.String("jwt.kid", kid),
		attribute.Bool("jwks.force_refresh", force),
	)

	cachedBytes, err := GetCachedTIDKeys(ctx, cfgAuth, redisClient, force)
	if err != nil {
		telemetry.RecordSpanError(span, err)
		return EntraIDKey{}, fmt.Errorf("failed to get tid keys: %w", err)
	}

	var keys EntraIDResponse
	err = json.Unmarshal(cachedBytes, &keys)
	if err != nil {
		telemetry.RecordSpanError(span, err)
		return EntraIDKey{}, fmt.Errorf("failed to unmarshal EntraID response: %w", err)
	}

	key, err := searchEntraIDKey(kid, keys)
	if err != nil {
		telemetry.RecordSpanError(span, err)
		return EntraIDKey{}, err
	}

	if !validateEntraIDKey(key) {
		err := fmt.Errorf("invalid JWKS key")
		telemetry.RecordSpanError(span, err)
		return EntraIDKey{}, fmt.Errorf("invalid JWKS key")
	}

	return key, nil
}

func ValidateEntraJWT(ctx context.Context, jwtToken string, cfg config.AuthConfig, redisClient *redis.RedisClient) (*ClaimsEntraID, error) {
	ctx, span := tracer.Start(ctx, "Auth.ValidateEntraJWT", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	start := time.Now()
	defer func() {
		span.SetAttributes(
			attribute.Float64("auth.duration_ms", float64(time.Since(start).Milliseconds())),
		)
	}()

	span.SetAttributes(
		attribute.String("tenant.id", cfg.TenantID),
		attribute.String("auth.issuer", cfg.Issuer),
	)

	validate := func(force bool) (*jwt.Token, error) {

		// no validation for NBF because its the same as IAT
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
			// JWT Header Validation
			func(token *jwt.Token) (any, error) {

				if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
					return nil, fmt.Errorf(
						"unexpected signing algorithm: %s",
						token.Header["alg"],
					)
				}

				kid, ok := token.Header["kid"].(string)
				if !ok || kid == "" {
					return nil, fmt.Errorf("missing kid in token header")
				}

				span.SetAttributes(attribute.String("jwt.kid", kid))

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
		span.SetAttributes(attribute.Bool("jwks.retry_performed", false))
		return finalizeClaims(token, cfg)
	}

	retryNeeded :=
		isSignatureError(err) ||
			strings.Contains(err.Error(), "key not found")

	if !retryNeeded {
		telemetry.RecordSpanError(span, err)
		return nil, fmt.Errorf("jwt validation failed: %w", err)
	}

	span.AddEvent("jwks_retry_after_signature_error")
	span.SetAttributes(attribute.Bool("jwks.retry_performed", true))

	// SECOND ATTEMPT – force refresh JWKS
	token, err = validate(true)
	if err != nil {
		telemetry.RecordSpanError(span, err)
		return nil, fmt.Errorf("jwt validation failed after JWKS refresh: %w", err)
	}

	return finalizeClaims(token, cfg)
}
