package kosync

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2/log"
	"github.com/joho/godotenv"
)

const ConfigFileName = "./kosync.env"

// ConfigFieldListenAddress TODO: Remove when 'legacy' is removed
const ConfigFieldListenAddress = "LISTEN_ADDRESS"

// ConfigFieldDebugLog TODO: Remove when 'legacy' is removed
const ConfigFieldDebugLog = "DEBUG_LOG"

// ConfigFieldDisableRegistration TODO: Remove when 'legacy' is removed
const ConfigFieldDisableRegistration = "DISABLE_REGISTRATION"

// ConfigFieldEnableWebUi TODO: Remove when 'legacy' is removed
const ConfigFieldEnableWebUi = "ENABLE_WEBUI"

type Config struct {
	DatabaseFile         string `env:"DATABASE_FILE" default:"./kosync.db"`
	ListenAddress        string `env:"LISTEN_ADDRESS" default:":8080"`
	PrintDebugLog        bool   `env:"DEBUG_LOG" default:"false"`
	DisableRegistration  bool   `env:"DISABLE_REGISTRATION" default:"false"`
	EnableWebUi          bool   `env:"ENABLE_WEBUI" default:"false"`
	EnableTrustedProxies bool   `env:"ENABLE_TRUSTED_PROXIES" default:"false"`
	TrustedProxies       string `env:"TRUSTED_PROXIES" default:""`
}

func NewConfig(fallback *Config) *Config {
	_ = godotenv.Overload(ConfigFileName)

	conf := Config{}

	sv := reflect.ValueOf(&conf)
	e := sv.Elem()
	for i := 0; i < e.NumField(); i++ {
		field := e.Field(i)
		fieldType := e.Type().Field(i)
		if alias, ok := fieldType.Tag.Lookup("env"); ok {
			if alias == "" {
				continue
			}
			if field.Kind() == reflect.Bool {
				fieldFallback := false
				if def, ok := fieldType.Tag.Lookup("default"); ok {
					fieldFallback, _ = strconv.ParseBool(strings.ToLower(def))
				}
				if field.CanSet() {
					field.SetBool(GetEnvBool(alias, fieldFallback))
				} else {
					println("Cannot set bool value for field: " + fieldType.Name)
				}
				continue
			} else if field.Kind() == reflect.String {
				fieldFallback := ""
				if def, ok := fieldType.Tag.Lookup("default"); ok {
					fieldFallback = def
				}
				if field.CanSet() {
					field.SetString(GetEnv(alias, fieldFallback))
				} else {
					println("Cannot set string value for field: " + fieldType.Name)
				}
				continue
			} else {
				panic(fmt.Sprintf("Config: Unsupported type '%s' of field '%s'.", fieldType.Type.Name(), fieldType.Name))
			}
		}
	}

	if fallback != nil {
		conf.EnableWebUi = conf.EnableWebUi || fallback.EnableWebUi
	}

	if conf.PrintDebugLog {
		log.Debugf("Loaded Config: %+v\n", conf)
	}

	return &conf
}

func GetEnv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

func GetEnvBool(key string, fallback bool) bool {
	if s, err := strconv.ParseBool(strings.ToLower(GetEnv(key, "false"))); err != nil {
		return fallback
	} else {
		return s
	}
}
