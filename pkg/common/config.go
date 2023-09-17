package common

import (
	"context"
	"fmt"
	"io"

	"github.com/sethvargo/go-envconfig"
	"gopkg.in/yaml.v3"
)

// Config struct contains configurations for SQUIRRELUP.
// Currently it contains two sections:
//   - S3 configuration
//   - Encryption configuration
type Config struct {
	S3 struct {
		Region string `yaml:"region" env:"SQUIRRELUP_S3_REGION"`
		ID     string `yaml:"id" env:"SQUIRRELUP_S3_ID" default:""`
		Secret string `yaml:"secret" env:"SQUIRRELUP_S3_SECRET" default:""`
		Token  string `yaml:"token" env:"SQUIRRELUP_S3_TOKEN" default:""`
	} `yaml:"s3"`
	Encryption struct {
		Pubkey string `yaml:"pubkey" env:"SQUIRRELUP_PUBKEY" default:""`
	} `yaml:"encryption"`
}

// LoadConfigFromFile loads configuration into a `Config` struct
// from YAML file.
func LoadConfigFromFile(cfg *Config, file io.Reader) error {
	decoder := yaml.NewDecoder(file)
	err := decoder.Decode(cfg)
	if err != nil {
		return fmt.Errorf("LoadConfigFromFile failed: %s", err.Error())
	}
	return nil
}

// LoadConfigFromEnv loads configuration into a `Config` struct
// from environment variables.
func LoadConfigFromEnv(cfg *Config) error {
	err := envconfig.Process(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("LoadConfigFromEnv failed: %s", err.Error())
	}
	return nil
}
