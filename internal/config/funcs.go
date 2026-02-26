package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"
)

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

	schemaToGUID := make(map[string]string)

	if err := json.NewDecoder(file).Decode(&schemaToGUID); err != nil {
		return nil, fmt.Errorf("decode schema config: %w", err)
	}

	guidToSchema := make(map[string]string, len(schemaToGUID))

	for schema, guid := range schemaToGUID {
		if _, err := uuid.Parse(guid); err != nil {
			return nil, fmt.Errorf("invalid GUID %s", guid)
		}

		if _, exists := guidToSchema[guid]; exists {
			return nil, fmt.Errorf("duplicate GUID detected: %s", guid)
		}

		guidToSchema[guid] = schema
	}

	return &SchemaConfig{
		SchemaToGUID: schemaToGUID,
		GUIDToSchema: guidToSchema,
	}, nil
}
