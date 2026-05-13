//
// File:        internal/kosync/rpc_util.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import "errors"

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

func getRpcArgumentInt64(arguments map[string]any, key string) (int64, error) {
	val, found := arguments[key]
	if !found {
		return 0, errors.New("RPC call is missing the argument '" + key + "'")
	}
	res, ok := getRpcInt64(val)
	if !ok {
		return 0, errors.New("RPC call has invalid type for argument '" + key + "'")
	}
	return res, nil
}

func getRpcArgumentString(arguments map[string]any, key string) (string, error) {
	val, found := arguments[key]
	if !found {
		return "", errors.New("RPC call is missing the argument '" + key + "'")
	}
	res, ok := val.(string)
	if !ok {
		return "", errors.New("RPC call has invalid type for argument '" + key + "'")
	}
	return res, nil
}
