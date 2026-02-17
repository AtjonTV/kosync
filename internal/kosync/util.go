//
// File:        internal/kosync/util.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func DecodeStructFromMap(data map[string]interface{}, dest interface{}, aliasName string) error {
	return DecodeStruct(&dest, "json", func(field *reflect.StructField, alias *string) (interface{}, bool) {
		val, found := data[aliasName]
		return val, found
	})
}

func DecodeStructFromEnv(dest interface{}) error {
	return DecodeStruct(&dest, "env", func(field *reflect.StructField, alias *string) (ret interface{}, has bool) {
		var err error
		defStr, hasDef := field.Tag.Lookup("default")

		if field.Type.Kind() == reflect.Bool {
			var def bool
			if hasDef {
				def, err = strconv.ParseBool(strings.ToLower(defStr))
				if err != nil {
					return nil, false
				}
			}
			return GetEnvBool(*alias, def), true
		} else if field.Type.Kind() == reflect.String {
			var def string
			if hasDef {
				def = defStr
			}
			return GetEnv(*alias, def), true
		} else if field.Type.Kind() == reflect.Int {
			var def int
			if hasDef {
				def, err = strconv.Atoi(defStr)
				if err != nil {
					return nil, false
				}
			}
			return GetEnvInt(*alias, def), true
		}

		return nil, false
	})
}

func DecodeStruct(dest *interface{}, aliasTag string, valueFunc func(field *reflect.StructField, aliasName *string) (interface{}, bool)) error {
	// Get a referential reflection of dest
	destReflect := reflect.ValueOf(*dest)
	// Writable instance of destReflect
	destReValue := destReflect.Elem()

	// force a tag name to be specified
	if len(aliasTag) == 0 {
		return errors.New("aliasTag must be set")
	}

	for i := 0; i < destReValue.NumField(); i++ {
		field := destReValue.Field(i)
		fieldType := destReValue.Type().Field(i)
		// Only process fields with the expected tag, ignore all others
		if alias, ok := fieldType.Tag.Lookup(aliasTag); ok {
			// Require the tag to be non-empty
			if alias == "" {
				continue
			}
			// Ignore field if it is read-only
			if !field.CanSet() {
				continue
			}

			// Get the value from the valueFunc
			val, found := valueFunc(&fieldType, &alias)
			if !found {
				continue
			}

			valType := reflect.TypeOf(val).Kind()

			// Match on type and set value
			if field.Kind() == reflect.Bool && valType == reflect.Bool {
				field.SetBool(val.(bool))
				continue
			}
			if field.Kind() == reflect.String && valType == reflect.String {
				field.SetString(val.(string))
				continue
			}
			if field.Kind() == reflect.Int && valType == reflect.Int {
				field.SetInt(int64(val.(int)))
				continue
			}
			return errors.New(fmt.Sprintf("unsupported type '%s' for field '%s'", fieldType.Type, fieldType.Name))
		}
	}
	return nil
}
