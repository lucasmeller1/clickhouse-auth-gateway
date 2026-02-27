package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/google/uuid"
)

func mustEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		slog.Error(fmt.Sprintf("env %s must not be empty", name))
		os.Exit(1)
	}
	return value
}

func mustConvertStringToIntEnv(s string) int {
	number, err := strconv.Atoi(os.Getenv(s))
	if err != nil {
		slog.Error(fmt.Sprintf("failed to convert %s", s), "error", err)
		os.Exit(1)
	}
	return number
}

func (sc *SchemaConfig) LookupSchemaByGUID(s string) (string, bool) {
	schema, ok := sc.GUIDToSchema[s]
	return schema, ok
}

func (sc *SchemaConfig) LookupGUIDBySchema(s string) (string, bool) {
	guid, ok := sc.SchemaToGUID[s]
	return guid, ok
}

func LoadSchemaConfig(path string) (*SchemaConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open schema config: %w", err)
	}
	defer file.Close()

	var schemasJSON struct {
		SchemasGUID   map[string]string `json:"schemas_guid"`
		PublicSchemas []string          `json:"public_schemas"`
	}
	if err := json.NewDecoder(file).Decode(&schemasJSON); err != nil {
		return nil, fmt.Errorf("decode schema config: %w", err)
	}

	guidToSchema := make(map[string]string, len(schemasJSON.SchemasGUID))
	for schema, guid := range schemasJSON.SchemasGUID {
		if _, err := uuid.Parse(guid); err != nil {
			return nil, fmt.Errorf("invalid GUID %s", guid)
		}
		if _, exists := guidToSchema[guid]; exists {
			return nil, fmt.Errorf("duplicate GUID detected: %s", guid)
		}
		guidToSchema[guid] = schema
	}

	return &SchemaConfig{
		SchemaToGUID:  schemasJSON.SchemasGUID,
		GUIDToSchema:  guidToSchema,
		PublicSchemas: schemasJSON.PublicSchemas,
	}, nil
}
