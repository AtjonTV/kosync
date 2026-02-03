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
	DebugLog             bool   `env:"DEBUG_LOG" default:"false"`
	DisableRegistration  bool   `env:"DISABLE_REGISTRATION" default:"false"`
	EnableWebUi          bool   `env:"ENABLE_WEBUI" default:"false"`
	EnableTrustedProxies bool   `env:"ENABLE_TRUSTED_PROXIES" default:"false"`
	TrustedProxies       string `env:"TRUSTED_PROXIES" default:""`
	EnableIPValidation   bool   `env:"ENABLE_IP_VALIDATION" default:"false"` // Only useful in combination with EnableTrustedProxies
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
	if s, err := strconv.ParseBool(strings.ToLower(GetEnv(key, strconv.FormatBool(fallback)))); err != nil {
		return fallback
	} else {
		return s
	}
}

func loadConfigFromEnvironment(conf *Config) {
	// Get a referential (pointer) reflection of conf, without "&" it would get a copy
	confReflect := reflect.ValueOf(conf)
	// Writable instance of confReflect
	confReValue := confReflect.Elem()
	for i := 0; i < confReValue.NumField(); i++ {
		field := confReValue.Field(i)
		fieldType := confReValue.Type().Field(i)
		// Only process fields with the env tag, ignore all others
		if alias, ok := fieldType.Tag.Lookup("env"); ok {
			// Require a tag to be actually present
			if alias == "" {
				continue
			}

			if field.Kind() == reflect.Bool {
				fieldDefault := false
				// Try to get a default
				if def, ok := fieldType.Tag.Lookup("default"); ok {
					fieldDefault, _ = strconv.ParseBool(strings.ToLower(def))
				}
				// Set if possible
				if field.CanSet() {
					field.SetBool(GetEnvBool(alias, fieldDefault))
				} else {
					println("Cannot set bool value for field: " + fieldType.Name)
				}
				continue
			} else if field.Kind() == reflect.String {
				fieldDefault := ""
				// Try to get a default
				if def, ok := fieldType.Tag.Lookup("default"); ok {
					fieldDefault = def
				}
				// Set if possible
				if field.CanSet() {
					field.SetString(GetEnv(alias, fieldDefault))
				} else {
					println("Cannot set string value for field: " + fieldType.Name)
				}
				continue
			} else {
				panic(fmt.Sprintf("Config: Unsupported type '%s' of field '%s'.", fieldType.Type.Name(), fieldType.Name))
			}
		}
	}
}
