package config

import (
	"time"
)

var SchemaToGUID = map[string]string{
	"Contabil_1": "11111111-1111-1111-1111-111111111111",
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

var GUIDToSchema = func() map[string]string {
	m := make(map[string]string, len(SchemaToGUID))
	for schema, guid := range SchemaToGUID {
		if _, exists := m[guid]; exists {
			panic("duplicate GUID detected: " + guid)
		}
		m[guid] = schema
	}
	return m
}()

type AuthConfig struct {
	TenantID string
	Issuer   string
	Audience string
}

type HTTPConfig struct {
	Addr              string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int

	ShutdownTimeout time.Duration
}

type PublicSchemasConfig struct {
	Schemas []string
}

type ClickhouseConfig struct {
	User     string
	Password string
	Schema   string
	Hostname string

	ClientTimeout int
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type Config struct {
	HTTP          HTTPConfig
	Auth          AuthConfig
	PublicSchemas PublicSchemasConfig
	Clickhouse    ClickhouseConfig
	Redis         RedisConfig
}
