package conf

import "time"

// Bootstrap is the root config structure.
type Bootstrap struct {
	Server  *Server  `yaml:"server"`
	Data    *Data    `yaml:"data"`
	Jwt     *Jwt     `yaml:"jwt"`
	Auth    *Auth    `yaml:"auth"`
	Tracing *Tracing `yaml:"tracing"`
}

// Server config.
type Tracing struct {
	Enabled  bool    `yaml:"enabled"`
	Endpoint string  `yaml:"endpoint"`
	Insecure bool    `yaml:"insecure"`
	Sampler  float64 `yaml:"sampler"`
}

type Server struct {
	HTTP         *HTTP  `yaml:"http"`
	ReadTimeout  string `yaml:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout"`
}

// HTTP server config.
type HTTP struct {
	Network string        `yaml:"network"`
	Addr    string        `yaml:"addr"`
	Timeout time.Duration `yaml:"timeout"`
}

// Data config.
type Data struct {
	Database *Database `yaml:"database"`
	Redis    *Redis    `yaml:"redis"`
}

// Database config.
type Database struct {
	Driver       string `yaml:"driver"`
	Source       string `yaml:"source"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

// Redis config.
type Redis struct {
	Network      string        `yaml:"network"`
	Addr         string        `yaml:"addr"`
	Password     string        `yaml:"password"`
	DB           int           `yaml:"db"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

// JWT config.
type Jwt struct {
	Secret string `yaml:"secret"`
	Expire int64  `yaml:"expire"`
}

// Auth config.
type Auth struct {
	Whitelist []string `yaml:"whitelist"`
}
