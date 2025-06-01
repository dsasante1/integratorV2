package config

type Config struct {
    // Add your config fields here
}

func Load() (*Config, error) {
    return &Config{}, nil
} 