package config

import (
	"github.com/spf13/cast"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config represents an alias to viper config
type Config = viper.Viper

var DiggerConfig *Config

// New returns a new pointer to the config
func New() *Config {
	v := viper.New()
	v.SetEnvPrefix("DIGGER")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.SetDefault("port", 3000)
	v.SetDefault("usersvc_on", true)
	v.SetDefault("build_date", "null")
	v.SetDefault("deployed_at", time.Now().UTC().Format(time.RFC3339))
	v.SetDefault("max_concurrency_per_batch", "0")
	v.BindEnv()
	return v
}

func GetPort() int {
	port := cast.ToInt(os.Getenv("PORT"))
	if port == 0 {
		port = 3000
	}
	return port
}

func init() {
	cfg := New()
	cfg.AutomaticEnv()
	DiggerConfig = cfg
}
