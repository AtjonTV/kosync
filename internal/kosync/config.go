package kosync

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const ConfigFileName = "./kosync.env"

const ConfigFieldListenAddress = "LISTEN_ADDRESS"
const ConfigFieldDebugLog = "DEBUG_LOG"
const ConfigFieldDisableRegistration = "DISABLE_REGISTRATION"
const ConfigFieldEnableWebUi = "ENABLE_WEBUI"

type Config struct {
	ListenAddress       string
	PrintDebugLog       bool
	DisableRegistration bool
	EnableWebUi         bool
}

func NewConfig(override *Config) *Config {
	_ = godotenv.Overload(ConfigFileName)

	conf := &Config{
		ListenAddress:       GetEnv(ConfigFieldListenAddress, ":8080"),
		PrintDebugLog:       GetEnvBool(ConfigFieldDebugLog, false),
		DisableRegistration: GetEnvBool(ConfigFieldDisableRegistration, false),
		EnableWebUi:         GetEnvBool(ConfigFieldEnableWebUi, false),
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
