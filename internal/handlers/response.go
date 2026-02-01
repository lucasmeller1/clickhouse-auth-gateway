package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"

	"github.com/lucasmeller1/excel_api/internal/auth"
	"github.com/lucasmeller1/excel_api/internal/config"
)

type contextKey int

const ClaimsContextKey contextKey = iota

func ClaimsFromContext(ctx context.Context) (*auth.CustomClaims, bool) {
	claims, ok := ctx.Value(ClaimsContextKey).(*auth.CustomClaims)
	return claims, ok
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

type ErrorResponse struct {
	Error string `json:"error"`
}

func JsonError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	err := json.NewEncoder(w).Encode(ErrorResponse{Error: message})
	if err != nil {
		log.Println("json encode:", err)
	}
}

func LookupSchemaByGUID(s string) (string, bool) {
	schema, ok := config.GUIDToSchema[s]
	return schema, ok
}

func LookupGUIDBySchema(s string) (string, bool) {
	guid, ok := config.SchemaToGUID[s]
	return guid, ok
}

func ValidateDatabase(r *http.Request, publicSchemas []string) (int, error) {
	claims, ok := ClaimsFromContext(r.Context())
	if !ok {
		return http.StatusInternalServerError, errors.New("internal server error")
	}

	database := strings.TrimSpace(r.URL.Query().Get("database"))
	table := strings.TrimSpace(r.URL.Query().Get("table"))

	// because tables can be created at any time and will not be validate like the schemas,
	// there will be just a simple check for empty tables
	if database == "" || table == "" {
		return http.StatusBadRequest, errors.New("missing database or table")
	}

	// check if requested schema is from the public ones
	if slices.Contains(publicSchemas, database) {
		return http.StatusOK, nil
	}

	// get GUID from know schemas (sector_level)
	databaseGUID, ok := LookupGUIDBySchema(database)
	if !ok {
		return http.StatusBadRequest, errors.New("unknown database schema")
	}

	// check if the requested one exist in the claims
	if slices.Contains(claims.Groups, databaseGUID) {
		return http.StatusOK, nil
	}

	return http.StatusForbidden, errors.New("no permisson for database")
}
