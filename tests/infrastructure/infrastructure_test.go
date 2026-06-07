package infrastructure_test

import (
	"testing"

	tauconfig "github.com/tailored-agentic-units/protocol/config"

	"github.com/JaimeStill/herald/internal/config"
	"github.com/JaimeStill/herald/internal/infrastructure"
	"github.com/JaimeStill/herald/pkg/auth"
	"github.com/JaimeStill/herald/pkg/database"
	"github.com/JaimeStill/herald/pkg/storage"
)

func validConfig() *config.Config {
	return &config.Config{
		Auth: auth.Config{Mode: auth.ModeNone},
		Agent: tauconfig.AgentConfig{
			Name:   "test-agent",
			Format: "openai",
			Provider: &tauconfig.ProviderConfig{
				Name:    "ollama",
				BaseURL: "http://localhost:11434",
				Options: make(map[string]any),
			},
			Model: &tauconfig.ModelConfig{
				Name: "llama3.1:8b",
			},
		},
		Database: database.Config{
			Host:            "localhost",
			Port:            5432,
			Name:            "herald",
			User:            "herald",
			Password:        "herald",
			SSLMode:         "disable",
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: "15m",
			ConnTimeout:     "5s",
		},
		Storage: storage.Config{
			Endpoint:   "localhost:9000",
			AccessKey:  "heraldstore",
			SecretKey:  "heraldstorepass",
			BucketName: "documents",
		},
		Version: "0.1.0",
	}
}

func TestNew(t *testing.T) {
	infra, err := infrastructure.New(validConfig())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if infra.Lifecycle == nil {
		t.Error("Lifecycle is nil")
	}
	if infra.Logger == nil {
		t.Error("Logger is nil")
	}
	if infra.Database == nil {
		t.Error("Database is nil")
	}
	if infra.Storage == nil {
		t.Error("Storage is nil")
	}
}

func TestNewAgentFactory(t *testing.T) {
	infra, err := infrastructure.New(validConfig())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if infra.NewAgent == nil {
		t.Fatal("NewAgent factory is nil")
	}

	a, err := infra.NewAgent(t.Context())
	if err != nil {
		t.Fatalf("NewAgent() error = %v", err)
	}

	if a.Model().Name != "llama3.1:8b" {
		t.Errorf("model name = %q, want %q", a.Model().Name, "llama3.1:8b")
	}

	if a.Provider().Name() != "ollama" {
		t.Errorf("provider name = %q, want %q", a.Provider().Name(), "ollama")
	}
}

func TestNewDatabaseConnection(t *testing.T) {
	infra, err := infrastructure.New(validConfig())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	conn := infra.Database.Connection()
	if conn == nil {
		t.Fatal("Database.Connection() returned nil")
	}
	conn.Close()
}

