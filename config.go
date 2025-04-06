// contains application server configuration
package main

import "time"

// ServerConfig contains all required server parameters
type ServerConfig struct {
	// server section
	Host string
	Port string

	// swagger section
	SwaggerEnabled bool
	SwaggerPort    string

	// sync section
	SrcPath        string
	DstPath        string
	MaxDiffPercent int

	// external data source
	VaultPath string

	// connection settings
	ConnReadTimeout         time.Duration
	ConnWriteTimeout        time.Duration
	GracefulShutdownTimeout time.Duration

	// CORS
	AllowedHosts   []string
	AllowedMethods []string
	AllowedHeaders []string

	// logger section
}
