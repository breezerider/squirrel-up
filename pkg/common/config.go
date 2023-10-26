package common

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/sethvargo/go-envconfig"
	"gopkg.in/yaml.v3"
)

// Config struct contains configurations for SQUIRRELUP.
// Currently it contains two sections:
//   - S3 configuration
//   - Encryption configuration
//   - Backup configuration
type Config struct {
	S3 struct {
		Region string `yaml:"region" env:"SQUIRRELUP_S3_REGION,overwrite" default:""`
		ID     string `yaml:"id" env:"SQUIRRELUP_S3_ID,overwrite" default:""`
		Secret string `yaml:"secret" env:"SQUIRRELUP_S3_SECRET,overwrite" default:""`
		Token  string `yaml:"token" env:"SQUIRRELUP_S3_TOKEN,overwrite" default:""`
	} `yaml:"s3"`
	Encryption struct {
		Pubkey string `yaml:"pubkey" env:"SQUIRRELUP_PUBKEY,overwrite" default:""`
	} `yaml:"encryption"`
	Backup struct {
		Hours float64 `yaml:"hours" env:"SQUIRRELUP_BACKUP_HOURS,overwrite" default:"240"`
		Name  string  `yaml:"name" env:"SQUIRRELUP_BACKUP_FILENAME,overwrite" default:"2006-01-02T15-0700"`
	} `yaml:"backup"`
}

func setDefaultValueField(valueof reflect.Value, tag string) error {
	switch valueof.Kind() {
	case reflect.String:
		valueof.SetString(tag)

	// case reflect.Int:
	// 	// fmt.Printf("reflect.Int\n")
	// 	if intValue, err := strconv.ParseInt(tag, 10, 64); err == nil {
	// 		valueof.SetInt(intValue)
	// 	}

	case reflect.Float64:
		if floatValue, err := strconv.ParseFloat(tag, 64); err == nil {
			valueof.SetFloat(floatValue)
		}
		// Add cases for other types if needed
		// default:
		// 	fmt.Printf("unknown\n")
		// 	return fmt.Errorf("setDefaultValueField called with an unsupported value of kind '%v'", valueof.Kind())
	}

	return nil
}

func setDefaultValuesStruct(valueof reflect.Value) error {
	typeinfo := valueof.Type()

	for i := 0; i < valueof.NumField(); i++ {
		field := valueof.Field(i)
		tag := typeinfo.Field(i).Tag.Get("default")

		// fmt.Printf("%s: default = %s, kind = %v\n", typeinfo.Field(i).Name, tag, field.Kind())

		if field.Kind() == reflect.Struct {
			if err := setDefaultValuesStruct(field); err != nil {
				return err
			}
			continue
		}

		if tag == "" {
			continue
		}

		if err := setDefaultValueField(field, tag); err != nil {
			return err
		}
	}

	return nil
}

// SetDefaultValues reset Config fields to default values.
// An implementation handling a generic struct:
//
//	func setDefaultValues() error {
//		if reflect.TypeOf(ptr).Kind() != reflect.Ptr {
//			return fmt.Errorf("Not a pointer")
//		}
//		valueof := reflect.ValueOf(ptr).Elem()
//		if ! valueof.IsValid() {
//			return fmt.Errorf("setDefaultValues called on a nil pointer")
//		}
//		fmt.Printf("setDefaultValues: kind = %v\n", valueof.Kind())
//		if valueof.Kind() == reflect.Struct {
//			if err := setDefaultValuesStruct(valueof); err != nil {
//				// fmt.Printf("setDefaultValues: err = %v\n", err)
//				return err
//			}
//			return nil
//		}
//		return fmt.Errorf("setDefaultValues called with unsupported value of kind '%v'", valueof.Kind())
//	}
func (cfg *Config) SetDefaultValues() error {
	valueof := reflect.ValueOf(cfg).Elem()

	if !valueof.IsValid() {
		return fmt.Errorf("SetDefaultValues called on a nil pointer")
	}

	return setDefaultValuesStruct(valueof)
}

// LoadConfigFromFile loads configuration into a `Config` struct
// from YAML file.
func (cfg *Config) LoadConfigFromFile(file io.Reader) error {
	decoder := yaml.NewDecoder(file)
	err := decoder.Decode(cfg)
	if err != nil {
		return fmt.Errorf("LoadConfigFromFile failed: %s", err.Error())
	}
	return nil
}

// LoadConfigFromEnv loads configuration into a `Config` struct
// from environment variables.
func (cfg *Config) LoadConfigFromEnv() error {
	err := envconfig.Process(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("LoadConfigFromEnv failed: %s", err.Error())
	}
	return nil
}
