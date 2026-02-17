//
// File:        internal/kosync/config.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"git.obth.eu/atjontv/kosync/pkg/decode"
	"github.com/joho/godotenv"
)

const ConfigFileName = "./kosync.env"

type Config struct {
	DatabaseFile string `env:"DATABASE_FILE" default:"./kosync.db"`

	ListenAddress       string `env:"LISTEN_ADDRESS" default:":8080"`
	DisableRegistration bool   `env:"DISABLE_REGISTRATION" default:"false"`

	LogToFile bool   `env:"LOG_TO_FILE" default:"false"`
	LogFile   string `env:"LOG_FILE" default:"./kosync.log"`
	DebugLog  bool   `env:"DEBUG_LOG" default:"false"`

	EnableWebUi        bool `env:"ENABLE_WEBUI" default:"false"`
	EnableWebSocketApi bool `env:"ENABLE_WEBSOCKET_API" default:"false"`

	EnableTrustedProxies bool   `env:"ENABLE_TRUSTED_PROXIES" default:"false"`
	TrustedProxies       string `env:"TRUSTED_PROXIES" default:""`
	ProxyIpValidation    bool   `env:"PROXY_IP_VALIDATION" default:"false"` // Only useful in combination with EnableTrustedProxies

	PrintCryptoKeys bool   `env:"PRINT_CRYPTO_KEYS" default:"false"`
	CryptoKeysSeed  string `env:"CRYPTO_KEYS_SEED" default:""`
	JwtDuration     int    `env:"JWT_DURATION" default:"21600"`
}

func NewConfig(fallback *Config) *Config {
	// Load environment variables from file
	_ = godotenv.Load(ConfigFileName)

	conf := Config{}

	loadConfigFromEnvironment(&conf)

	if fallback != nil {
		conf.EnableWebUi = conf.EnableWebUi || fallback.EnableWebUi
	}

	if conf.DebugLog {
		LogDebugUnchecked("Loaded Config: %+v\n", conf)
	}

	return &conf
}

func loadConfigFromEnvironment(conf *Config) {
	if err := decode.StructFromEnv(conf); err != nil {
		LogError("Failed to load config from environment: %v", err.Error())
	}
}
