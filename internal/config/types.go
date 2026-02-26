package config

import (
	"time"
)

type SchemaConfig struct {
	SchemaToGUID map[string]string
	GUIDToSchema map[string]string
}

type AuthConfig struct {
	TenantID string
	Issuer   string
	Audience string
	Debug    string
}

type ServerConfig struct {
	Addr              string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int

	ShutdownTimeout time.Duration

	MaxRequestsExportEDP         int
	MaxRequestsIntervalExportEDP time.Duration

	MaxRequestsTablesEDP         int
	MaxRequestsIntervalTablesEDP time.Duration
}

type ClickhouseConfig struct {
	User     string
	Password string
	Schema   string
	Hostname string

	ClientTimeout    int
	PublicSchemas    []string
	TransportConfig  HTTPTransportClickhouse
	TTLTablesInRedis time.Duration

	QueueSizeLimiter int
}

type HTTPTransportClickhouse struct {
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	IdleConnTimeout     time.Duration
}

type RedisConfig struct {
	Hostname string
	Port     int
	Password string
	DB       int
}

type Config struct {
	Server        ServerConfig
	Auth          AuthConfig
	Clickhouse    ClickhouseConfig
	Redis         RedisConfig
	PrivateServer PrivateServerConfig
	Endpoints     EndpointsConfig
	SchemasGUIDs  SchemaConfig
}

type PrivateServerConfig struct {
	InvalidateCacheToken string
}

type EndpointsConfig struct {
	Export  string
	Tables  string
	Version string
}
