package common

import (
	"os"
	"strings"
	"testing"
)

/* test cases for SetDefaultValues */
func TestSetDefaultValuesValid(t *testing.T) {
	cfg := new(Config)
	if cfg == nil {
		t.Fatalf("could not allocate memory")
	}

	if err := cfg.SetDefaultValues(); err != nil {
		t.Fatalf(err.Error())
	} else {
		assertEquals(t, "", cfg.S3.Region, "cfg.S3.Region")
		assertEquals(t, "", cfg.S3.ID, "cfg.S3.ID")
		assertEquals(t, "", cfg.S3.Secret, "cfg.S3.Secret")
		assertEquals(t, "", cfg.S3.Token, "cfg.S3.Token")
		assertEquals(t, 240.0, cfg.Backup.Hours, "cfg.Backup.Hours")
		assertEquals(t, "2006-01-02T15-0700", cfg.Backup.Name, "cfg.Backup.Name")
		assertEquals(t, "", cfg.Encryption.Pubkey, "cfg.Encryption.Pubkey")
	}
}

func TestSetDefaultValuesInvalid(t *testing.T) {
	var cfg *Config = nil

	if err := cfg.SetDefaultValues(); err == nil {
		t.Fatalf("This test should throw an error")
	} else {
		assertEquals(t, "SetDefaultValues called on a nil pointer", err.Error(), "err.Error")
	}
}

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

backup:
  hours: 0.1
  name: "test"

encryption:
  pubkey: "mock-pubkey"

`

	if err := cfg.LoadConfigFromFile(strings.NewReader(yaml)); err != nil {
		t.Fatalf(err.Error())
	} else {
		assertEquals(t, "mock-region", cfg.S3.Region, "cfg.S3.Region")
		assertEquals(t, "mock-id", cfg.S3.ID, "cfg.S3.ID")
		assertEquals(t, "mock-secret", cfg.S3.Secret, "cfg.S3.Secret")
		assertEquals(t, "mock-token", cfg.S3.Token, "cfg.S3.Token")
		assertEquals(t, 0.1, cfg.Backup.Hours, "cfg.Backup.Hours")
		assertEquals(t, "test", cfg.Backup.Name, "cfg.Backup.Name")
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

	if err := cfg.LoadConfigFromFile(strings.NewReader(yaml)); err == nil {
		t.Fatalf("This test should throw an error")
	} else {
		assertEquals(t, "LoadConfigFromFile failed: yaml: line 2: mapping values are not allowed in this context", err.Error(), "err.Error")
	}
}

/* test cases for LoadConfigFromEnv */
func TestLoadConfigFromEnvValid(t *testing.T) {
	cfg := new(Config)
	if cfg == nil {
		t.Fatalf("could not allocate memory")
	}
	os.Setenv("SQUIRRELUP_S3_REGION", "mock-region")
	os.Setenv("SQUIRRELUP_S3_ID", "mock-id")
	os.Setenv("SQUIRRELUP_S3_SECRET", "mock-secret")
	os.Setenv("SQUIRRELUP_S3_TOKEN", "mock-token")
	os.Setenv("SQUIRRELUP_BACKUP_HOURS", "0.1")
	os.Setenv("SQUIRRELUP_BACKUP_FILENAME", "test")
	os.Setenv("SQUIRRELUP_PUBKEY", "mock-pubkey")

	if err := cfg.LoadConfigFromEnv(); err != nil {
		t.Fatalf(err.Error())
	} else {
		assertEquals(t, "mock-region", cfg.S3.Region, "cfg.S3.Region")
		assertEquals(t, "mock-id", cfg.S3.ID, "cfg.S3.ID")
		assertEquals(t, "mock-secret", cfg.S3.Secret, "cfg.S3.Secret")
		assertEquals(t, "mock-token", cfg.S3.Token, "cfg.S3.Token")
		assertEquals(t, 0.1, cfg.Backup.Hours, "cfg.Backup.Hours")
		assertEquals(t, "test", cfg.Backup.Name, "cfg.Backup.Name")
		assertEquals(t, "mock-pubkey", cfg.Encryption.Pubkey, "cfg.Encryption.Pubkey")
	}
}

func TestLoadConfigFromEnvInvalid(t *testing.T) {
	cfg := new(Config)
	if cfg == nil {
		t.Fatalf("could not allocate memory")
	}
	os.Setenv("SQUIRRELUP_BACKUP_HOURS", "invalid")

	if err := cfg.LoadConfigFromEnv(); err == nil {
		t.Fatalf("This test should throw an error")
	} else {
		assertEquals(t, `LoadConfigFromEnv failed: Backup: Hours("invalid"): strconv.ParseFloat: parsing "invalid": invalid syntax`, err.Error(), "err.Error")
	}
}
