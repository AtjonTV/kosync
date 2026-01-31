package kosync

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	ListenAddress       string
	PrintDebugLog       bool
	DisableRegistration bool
	EnableWebUi         bool
}

func NewConfig(override *Config) *Config {
	godotenv.Load("kosync.env")

	conf := &Config{
		ListenAddress:       GetEnv("LISTEN_ADDRESS", ":8080"),
		PrintDebugLog:       GetEnvBool("DEBUG_LOG", false),
		DisableRegistration: GetEnvBool("DISABLE_REGISTRATION", false),
		EnableWebUi:         GetEnvBool("ENABLE_WEBUI", false),
	}

	if override != nil {
		conf.EnableWebUi = override.EnableWebUi
	}

	return conf
}

func GetEnv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

func GetEnvBool(key string, fallback bool) bool {
	lowerEnv := strings.ToLower(GetEnv(key, "false"))
	return lowerEnv == "true" || lowerEnv == "yes" || lowerEnv == "1" || lowerEnv == "on" || lowerEnv == "t" || lowerEnv == "y" || fallback
}
