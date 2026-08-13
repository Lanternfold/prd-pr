package config

// Config holds user and project settings.
// P0 keeps this empty; later phases load flags, env, and YAML.
type Config struct{}

// Defaults returns the empty P0 configuration.
func Defaults() Config {
	return Config{}
}
