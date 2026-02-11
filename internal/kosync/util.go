package kosync

import (
	"reflect"
)

func DecodeStructFromMap(data map[string]interface{}, dest interface{}, aliasTag string) {
	// Get a referential (pointer) reflection of dest
	destReflect := reflect.ValueOf(dest)
	// Writable instance of destReflect
	destReValue := destReflect.Elem()

	tagName := "json"
	if len(aliasTag) > 0 {
		tagName = aliasTag
	}

	for i := 0; i < destReValue.NumField(); i++ {
		field := destReValue.Field(i)
		fieldType := destReValue.Type().Field(i)
		// Only process fields with the env tag, ignore all others
		if alias, ok := fieldType.Tag.Lookup(tagName); ok {
			// Require a tag to be actually present
			if alias == "" {
				continue
			}
			if !field.CanSet() {
				continue
			}

			val, found := data[alias]
			if !found {
				continue
			}

			valType := reflect.TypeOf(val).Kind()

			if field.Kind() == reflect.Bool && valType == reflect.Bool {
				field.SetBool(val.(bool))
				continue
			}
			if field.Kind() == reflect.String && valType == reflect.String {
				field.SetString(val.(string))
				continue
			}
		}
	}
}
