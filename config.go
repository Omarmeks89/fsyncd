// contains application server configuration
package main

import (
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert/yaml"
	"io"
	"os"
	"sync"
	"time"
)

// DefaultConfigName for detect config file
const DefaultConfigName = "fsync.yml"

// ServerConfig contains all required server parameters
type ServerConfig struct {
	// server section
	Host string `yaml:"host" validate:"required,ipv4"`
	Port string `yaml:"port" validate:"required,numeric"`

	// swagger section
	SwaggerEnabled bool   `yaml:"swagger_enabled" validate:"required"`
	SwaggerPort    string `yaml:"swagger_port" validate:"numeric"`

	// sync section
	// 'dirpath' will fail if directory not exists
	SrcPath        string `yaml:"src_path" validate:"required,dirpath"`
	DstPath        string `yaml:"dst_path" validate:"required,dirpath"`
	MaxDiffPercent int    `yaml:"max_diff_percent" validate:"required,gt=0,lte=100"`
	SyncTime       string `yaml:"sync_time" validate:"required"`

	// external data source
	// ...

	// connection settings
	ConnReadTimeout         time.Duration `yaml:"conn_read_timeout" validate:"required"`
	ConnWriteTimeout        time.Duration `yaml:"conn_write_timeout" validate:"required"`
	GracefulShutdownTimeout time.Duration `yaml:"graceful_shutdown_timeout" validate:"required"`

	// CORS
	AllowedHosts   []string `yaml:"allowed_hosts" validate:"required"`
	AllowedMethods []string `yaml:"allowed_methods" validate:"required"`
	AllowedHeaders []string `yaml:"allowed_headers" validate:"required"`

	// logger section
	TimeFormat string `yaml:"time_format" validate:"required"`
	LogLevel   string `yaml:"log_level" validate:"required"`

	lock *sync.RWMutex
}

// Load parameters from config file and setup config
func (sc *ServerConfig) Load() (err error) {
	var file *os.File
	var buf []byte

	if file, err = os.Open(DefaultConfigName); err != nil {
		return err
	}

	if buf, err = io.ReadAll(file); err != nil {
		return err
	}

	if err = yaml.Unmarshal(buf, sc); err != nil {
		return err
	}

	if _, err = sc.Validate(); err != nil {
		return err
	}

	return err
}

// Validate config fields and return error if validation failed
func (sc *ServerConfig) Validate() (ok bool, err error) {
	v := validator.New(validator.WithRequiredStructEnabled())
	if err = v.Struct(sc); err != nil {
		return ok, err
	}

	return ok, err
}
