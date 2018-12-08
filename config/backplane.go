package config

import (
	"github.com/choria-io/prometheus-streams/build"
)

// FactData implements backplane.InfoSource
func (c *Config) FactData() interface{} {
	return c
}

// Version implements backplane.InfoSource
func (c *Config) Version() string {
	return build.Version
}
