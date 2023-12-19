package config

import (
	"time"

	"github.com/spf13/viper"
)

// Config represents an alias to viper config
type Config = viper.Viper

// New returns a new pointer to the config
func New() *Config {
	v := viper.New()
	v.SetDefault("port", 3000)
	v.SetDefault("usersvc_on", true)
	v.SetDefault("build_date", "null")
	v.SetDefault("deployed_at", time.Now().UTC().Format(time.RFC3339))
	return v
}
