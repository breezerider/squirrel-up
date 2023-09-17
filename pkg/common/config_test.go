package common

import (
	"os"
	"strings"
	"testing"
)

/* test cases for LoadConfigFromFile */
func TestLoadConfigFromFileValid(t *testing.T) {
	cfg := new(Config)
	if cfg == nil {
		t.Fatalf("could not allocate memory")
	}
	yaml := `s3:
  region: "mock-region"
  id: "mock-id"
  secret: "mock-secret"
  token: "mock-token"

encryption:
  pubkey: "mock-pubkey"

`

	err := LoadConfigFromFile(cfg, strings.NewReader(yaml))
	if err != nil {
		t.Fatalf(err.Error())
	} else {
		assertEquals(t, "mock-region", cfg.S3.Region, "cfg.S3.Region")
		assertEquals(t, "mock-id", cfg.S3.ID, "cfg.S3.ID")
		assertEquals(t, "mock-secret", cfg.S3.Secret, "cfg.S3.Secret")
		assertEquals(t, "mock-token", cfg.S3.Token, "cfg.S3.Token")
		assertEquals(t, "mock-pubkey", cfg.Encryption.Pubkey, "cfg.Encryption.Pubkey")
	}
}

func TestLoadConfigFromFileInvalid(t *testing.T) {
	cfg := new(Config)
	if cfg == nil {
		t.Fatalf("could not allocate memory")
	}
	yaml := `test:
  a: b: c:
`

	err := LoadConfigFromFile(cfg, strings.NewReader(yaml))
	if err == nil {
		t.Fatalf("This test should throw an error")
	} else {
		assertEquals(t, "LoadConfigFromFile failed: yaml: line 2: mapping values are not allowed in this context", err.Error(), "err.Error")
	}
}

/* test cases for LoadConfigFromEnv */
func TestLoadConfigFromEnv(t *testing.T) {
	cfg := new(Config)
	if cfg == nil {
		t.Fatalf("could not allocate memory")
	}
	os.Setenv("SQUIRRELUP_S3_REGION", "mock-region")
	os.Setenv("SQUIRRELUP_S3_ID", "mock-id")
	os.Setenv("SQUIRRELUP_S3_SECRET", "mock-secret")
	os.Setenv("SQUIRRELUP_S3_TOKEN", "mock-token")
	os.Setenv("SQUIRRELUP_PUBKEY", "mock-pubkey")

	err := LoadConfigFromEnv(cfg)
	if err != nil {
		t.Fatalf(err.Error())
	} else {
		assertEquals(t, "mock-region", cfg.S3.Region, "cfg.S3.Region")
		assertEquals(t, "mock-id", cfg.S3.ID, "cfg.S3.ID")
		assertEquals(t, "mock-secret", cfg.S3.Secret, "cfg.S3.Secret")
		assertEquals(t, "mock-token", cfg.S3.Token, "cfg.S3.Token")
		assertEquals(t, "mock-pubkey", cfg.Encryption.Pubkey, "cfg.Encryption.Pubkey")
	}
}
