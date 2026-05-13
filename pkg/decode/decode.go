//
// File:        pkg/decode/decode.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package decode

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"git.obth.eu/atjontv/kosync/pkg/environ"
)

func StructFromMap(dest any, aliasName string, data map[string]interface{}) error {
	return StructRaw(&dest, aliasName, func(field *reflect.StructField, alias *string) (interface{}, bool) {
		val, found := data[*alias]
		// If the field is not a pointer, construct an empty instance if val is nil
		if val == nil && field.Type.Kind() != reflect.Ptr {
			return reflect.New(field.Type).Elem().Interface(), found
		}
		return val, found
	})
}

func getEnvValue(field *reflect.StructField, alias *string) (interface{}, bool) {
	var err error
	defStr, hasDef := field.Tag.Lookup("default")

	switch field.Type.Kind() {
	case reflect.Bool:
		var def bool
		if hasDef {
			def, err = strconv.ParseBool(strings.ToLower(defStr))
			if err != nil {
				return nil, false
			}
		}
		return environ.GetEnvBool(*alias, def), true
	case reflect.String:
		var def string
		if hasDef {
			def = defStr
		}
		return environ.GetEnv(*alias, def), true
	case reflect.Int:
		var def int
		if hasDef {
			def, err = strconv.Atoi(defStr)
			if err != nil {
				return nil, false
			}
		}
		return environ.GetEnvInt(*alias, def), true
	default:
	}

	return nil, false
}

func StructFromEnv(dest any) error {
	return StructRaw(&dest, "env", getEnvValue)
}

func StructRaw(dest *any, aliasTag string, valueFunc func(field *reflect.StructField, aliasName *string) (interface{}, bool)) (err error) {
	defer func() {
		// recover from panic if one occurred.
		e := recover()
		if e != nil {
			err = fmt.Errorf("%+v", e)
		}
	}()

	// Get a referential reflection of dest
	destReflect := reflect.ValueOf(*dest)
	// Writable instance of destReflect
	destReValue := destReflect.Elem()

	// force a tag name to be specified
	if len(aliasTag) == 0 {
		return errors.New("aliasTag must be set")
	}

	for i := range destReValue.NumField() {
		field := destReValue.Field(i)
		fieldType := destReValue.Type().Field(i)
		// Only process fields with the expected tag, ignore all others
		alias, ok := fieldType.Tag.Lookup(aliasTag)
		if !ok || alias == "" {
			continue
		}
		// Ignore field if it is read-only
		if !field.CanSet() {
			continue
		}

		// Ignore everything after a comma (needed when a field has ",omitempty")
		if strings.Contains(alias, ",") {
			alias = strings.Split(alias, ",")[0]
		}

		// Get the value from the valueFunc
		val, found := valueFunc(&fieldType, &alias)
		if !found {
			continue
		}

		// Reject nil value when field cant hold nil
		if val == nil && field.Kind() != reflect.Ptr {
			return fmt.Errorf("value is nil for field '%s'", fieldType.Name)
		}

		// Set the field value to val
		field.Set(reflect.ValueOf(val))
	}
	return nil
}
