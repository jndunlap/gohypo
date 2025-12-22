package config

import (
	"os"
	"strconv"
	"time"

	"gohypo/internal/errors"
)

// Config represents the complete application configuration
type Config struct {
	Database  DatabaseConfig `validate:"required"`
	AI        AIConfig       `validate:"required"`
	Server    ServerConfig   `validate:"required"`
	Paths     PathConfig     `validate:"required"`
	Data      DataConfig     `validate:"required"`
	Profiling ProfilingConfig
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	URL      string `validate:"required"`
	User     string
	Password string
	Name     string
	Host     string
	Port     int
	SSLMode  string
}

// AIConfig holds AI/LLM related settings
type AIConfig struct {
	OpenAIKey     string `validate:"required"`
	OpenAIModel   string `validate:"required"`
	SystemContext string
	MaxTokens     int
	Temperature   float64
	PromptsDir    string `validate:"required"`
}

// ServerConfig holds web server settings
type ServerConfig struct {
	Port    string `validate:"required"`
	GinMode string
}

// PathConfig holds file system paths
type PathConfig struct {
	ExcelFile string
}

// DataConfig holds data processing settings
type DataConfig struct {
	ExcelFile string
}

// ProfilingConfig holds performance profiling settings
type ProfilingConfig struct {
	Port    string
	Enabled bool
}

// Load reads configuration from environment variables and validates it
func Load() (*Config, error) {
	config := &Config{}

	// Load database configuration
	dbConfig, err := loadDatabaseConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to load database configuration")
	}
	config.Database = *dbConfig

	// Load AI configuration
	aiConfig, err := loadAIConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to load AI configuration")
	}
	config.AI = *aiConfig

	// Load server configuration
	serverConfig := loadServerConfig()
	config.Server = *serverConfig

	// Load path configuration
	pathConfig := loadPathConfig()
	config.Paths = *pathConfig

	// Load data configuration
	dataConfig := loadDataConfig()
	config.Data = *dataConfig

	// Load profiling configuration
	profilingConfig := loadProfilingConfig()
	config.Profiling = *profilingConfig

	// Validate required fields
	if err := validateConfig(config); err != nil {
		return nil, errors.Wrap(err, "configuration validation failed")
	}

	return config, nil
}

func loadDatabaseConfig() (*DatabaseConfig, error) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return nil, errors.ConfigInvalid("DATABASE_URL is required")
	}

	return &DatabaseConfig{
		URL:      url,
		User:     getEnvOrDefault("DB_USER", ""),
		Password: getEnvOrDefault("DB_PASS", ""),
		Name:     getEnvOrDefault("DB_NAME", ""),
		Host:     getEnvOrDefault("DB_HOST", ""),
		Port:     getEnvIntOrDefault("DB_PORT", 5432),
		SSLMode:  getEnvOrDefault("SSL_MODE", "disable"),
	}, nil
}

func loadAIConfig() (*AIConfig, error) {
	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey == "" {
		return nil, errors.ConfigInvalid("OPENAI_API_KEY is required")
	}

	promptsDir := os.Getenv("PROMPTS_DIR")
	if promptsDir == "" {
		promptsDir = "./prompts" // default
	}

	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = "gpt-5.2turbo-preview" // default
	}

	return &AIConfig{
		OpenAIKey:     openaiKey,
		OpenAIModel:   model,
		SystemContext: "You are a statistical research assistant",
		MaxTokens:     getEnvIntOrDefault("MAX_TOKENS", 4000), // Reasonable default for gpt-5.2 (8192 context limit)
		Temperature:   getEnvFloatOrDefault("TEMPERATURE", 1.0),
		PromptsDir:    promptsDir,
	}, nil
}

func loadServerConfig() *ServerConfig {
	return &ServerConfig{
		Port:    getEnvOrDefault("PORT", "8080"),
		GinMode: getEnvOrDefault("GIN_MODE", "debug"),
	}
}

func loadPathConfig() *PathConfig {
	return &PathConfig{
		ExcelFile: getEnvOrDefault("EXCEL_FILE", ""),
	}
}

func loadDataConfig() *DataConfig {
	return &DataConfig{
		ExcelFile: getEnvOrDefault("EXCEL_FILE", ""),
	}
}

func loadProfilingConfig() *ProfilingConfig {
	return &ProfilingConfig{
		Port:    getEnvOrDefault("PPROF_PORT", "6060"),
		Enabled: getEnvBoolOrDefault("PPROF_ENABLED", true),
	}
}

func validateConfig(config *Config) error {
	if config.Database.URL == "" {
		return errors.ConfigInvalid("database URL is required")
	}
	if config.AI.OpenAIKey == "" {
		return errors.ConfigInvalid("OpenAI API key is required")
	}
	if config.AI.PromptsDir == "" {
		return errors.ConfigInvalid("prompts directory is required")
	}
	return nil
}

// Helper functions for environment variable parsing
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvFloatOrDefault(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// Duration parsing helper (for future use)
func getEnvDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
