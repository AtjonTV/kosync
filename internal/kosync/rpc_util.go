//
// File:        internal/kosync/rpc_util.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"fmt"
)

func getRpcInt64(val any) (int64, bool) {
	switch v := val.(type) {
	case int64:
		return v, true
	case float64:
		return int64(v), true
	case int:
		return int64(v), true
	default:
		return 0, false
	}
}

func getRpcArgumentInt64(lang Language, arguments map[string]any, key string) (int64, error) {
	val, found := arguments[key]
	if !found {
		return 0, fmt.Errorf(Translate(lang, "err_rpc_missing_argument"), key)
	}
	res, ok := getRpcInt64(val)
	if !ok {
		return 0, fmt.Errorf(Translate(lang, "err_rpc_invalid_argument_type"), key)
	}
	return res, nil
}

func getRpcArgumentString(lang Language, arguments map[string]any, key string) (string, error) {
	val, found := arguments[key]
	if !found {
		return "", fmt.Errorf(Translate(lang, "err_rpc_missing_argument"), key)
	}
	res, ok := val.(string)
	if !ok {
		return "", fmt.Errorf(Translate(lang, "err_rpc_invalid_argument_type"), key)
	}
	return res, nil
}
