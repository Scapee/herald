package storage

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds MinIO connection parameters.
type Config struct {
	Endpoint    string `json:"endpoint"`
	AccessKey   string `json:"access_key"`
	SecretKey   string `json:"secret_key"`
	UseSSL      bool   `json:"use_ssl"`
	BucketName  string `json:"bucket_name"`
	MaxListSize int32  `json:"max_list_size"`
}

// Env maps config fields to environment variable names for override injection.
type Env struct {
	Endpoint    string
	AccessKey   string
	SecretKey   string
	UseSSL      string
	BucketName  string
	MaxListSize string
}

// Finalize applies defaults, environment variable overrides, and validation.
func (c *Config) Finalize(env *Env) error {
	c.loadDefaults()
	if env != nil {
		c.loadEnv(env)
	}
	return c.validate()
}

// Merge overwrites non-zero fields from overlay.
func (c *Config) Merge(overlay *Config) {
	if overlay.Endpoint != "" {
		c.Endpoint = overlay.Endpoint
	}
	if overlay.AccessKey != "" {
		c.AccessKey = overlay.AccessKey
	}
	if overlay.SecretKey != "" {
		c.SecretKey = overlay.SecretKey
	}
	if overlay.UseSSL {
		c.UseSSL = true
	}
	if overlay.BucketName != "" {
		c.BucketName = overlay.BucketName
	}
	if overlay.MaxListSize != 0 {
		c.MaxListSize = overlay.MaxListSize
	}
}

func (c *Config) loadDefaults() {
	if c.BucketName == "" {
		c.BucketName = "documents"
	}
	if c.MaxListSize == 0 {
		c.MaxListSize = 50
	}
	if c.MaxListSize > MaxListCap {
		c.MaxListSize = MaxListCap
	}
}

func (c *Config) loadEnv(env *Env) {
	if env.Endpoint != "" {
		if v := os.Getenv(env.Endpoint); v != "" {
			c.Endpoint = v
		}
	}
	if env.AccessKey != "" {
		if v := os.Getenv(env.AccessKey); v != "" {
			c.AccessKey = v
		}
	}
	if env.SecretKey != "" {
		if v := os.Getenv(env.SecretKey); v != "" {
			c.SecretKey = v
		}
	}
	if env.UseSSL != "" {
		if v := os.Getenv(env.UseSSL); v != "" {
			if b, err := strconv.ParseBool(v); err == nil {
				c.UseSSL = b
			}
		}
	}
	if env.BucketName != "" {
		if v := os.Getenv(env.BucketName); v != "" {
			c.BucketName = v
		}
	}
	if env.MaxListSize != "" {
		if v := os.Getenv(env.MaxListSize); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				c.MaxListSize = min(int32(n), MaxListCap)
			}
		}
	}
}

func (c *Config) validate() error {
	if c.BucketName == "" {
		return fmt.Errorf("bucket_name required")
	}
	return nil
}
