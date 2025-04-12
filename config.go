// contains application server configuration
package main

import (
	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"sync"
	"time"
)

// DefaultConfigName for detect config file
const DefaultConfigName = "fsync.yml"
const DriverConfigFilename = "driver_config.yml"

// ServerConfig contains all required server parameters
type ServerConfig struct {
	// server section
	Host string `yaml:"host" validate:"required,ipv4"`
	Port string `yaml:"port" validate:"required,numeric"`

	// swagger section
	SwaggerEnabled bool   `yaml:"swagger_enabled" validate:"required"`
	SwaggerPort    string `yaml:"swagger_port" validate:"numeric"`

	ConfigDriver string `yaml:"config_driver" validate:"required,oneof=vault default"`

	Location string `yaml:"location" validate:"required"`

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

	if err = sc.Validate(); err != nil {
		return err
	}

	return err
}

// Validate config fields and return error if validation failed
func (sc *ServerConfig) Validate() (err error) {
	v := validator.New(validator.WithRequiredStructEnabled())
	if err = v.Struct(sc); err != nil {
		return err
	}

	return err
}

type SyncConfig struct {
	SrcPath        string `yaml:"src_path" json:"src_path" validate:"required,dirpath"`
	DstPath        string `yaml:"dst_path" json:"dst_path" validate:"required,dirpath,nefield=SrcPath"`
	MaxDiffPercent int    `yaml:"max_diff_percent" json:"max_diff_percent" validate:"required,gt=0,lte=100"`
	SyncTime       string `yaml:"sync_time" json:"sync_time" validate:"required"`
}

type DefaultConfigDriver struct{}

func (d DefaultConfigDriver) LoadSyncConfig() (
	c SyncConfig,
	err error,
) {
	var file *os.File
	var buf []byte

	if file, err = os.Open(DriverConfigFilename); err != nil {
		return c, err
	}

	if buf, err = io.ReadAll(file); err != nil {
		return c, err
	}

	if err = yaml.Unmarshal(buf, &c); err != nil {
		return c, err
	}

	if err = d.Validate(&c); err != nil {
		return c, err
	}

	return c, err
}

func (d DefaultConfigDriver) Validate(c *SyncConfig) (err error) {
	vld := validator.New(validator.WithRequiredStructEnabled())
	if err = vld.Struct(c); err != nil {
		return err
	}

	return err
}

func (d DefaultConfigDriver) UpdateSyncConfig(nc SyncConfig) (err error) {
	var buf []byte
	var file *os.File
	var stat os.FileInfo

	if err = d.Validate(&nc); err != nil {
		return err
	}

	if stat, err = os.Stat(DriverConfigFilename); err != nil {
		return err
	}

	// open file with same file mode
	if file, err = os.OpenFile(
		DriverConfigFilename,
		os.O_RDWR|os.O_TRUNC,
		stat.Mode(),
	); err != nil {
		return err
	}
	defer file.Close()

	if buf, err = yaml.Marshal(&nc); err != nil {
		return err
	}

	if _, err = file.Write(buf); err != nil {
		return err
	}

	return err
}
