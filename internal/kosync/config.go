package kosync

import (
	"os"
	"strconv"
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
	_ = godotenv.Load("kosync.env")

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
	if s, err := strconv.ParseBool(strings.ToLower(GetEnv(key, "false"))); err == nil {
		return fallback
	} else {
		return s
	}
}
