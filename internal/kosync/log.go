//
// File:        internal/kosync/userLog.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"io"
	"os"

	"github.com/gofiber/fiber/v3/log"
)

var doDebugLogging = false

type Klog struct {
	prefix string
}

func SetDebugLogging(enabled bool) {
	doDebugLogging = enabled
}

func SetLogOutput(writeToFile bool, filename string) {
	if writeToFile {
		f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			LogError("Failed to open log file '%s': %v", filename, err.Error())
			LogDebug("Continuing with stdout-only")
			return
		}
		log.SetOutput(io.MultiWriter(os.Stdout, f))
	}
}

func LogDebug(fmt string, args ...interface{}) {
	if doDebugLogging {
		if len(args) > 0 {
			log.Debugf(fmt, args...)
		} else {
			log.Debugf(fmt)
		}
	}
}

func LogDebugUnchecked(fmt string, args ...interface{}) {
	if len(args) > 0 {
		log.Debugf(fmt, args...)
	} else {
		log.Debugf(fmt)
	}
}

func LogError(fmt string, args ...interface{}) {
	if len(args) > 0 {
		log.Errorf(fmt, args...)
	} else {
		log.Errorf(fmt)
	}
}

func LogInfo(fmt string, args ...interface{}) {
	if len(args) > 0 {
		log.Infof(fmt, args...)
	} else {
		log.Infof(fmt)
	}
}

func NewKlog(tag string) *Klog {
	return &Klog{prefix: tag}
}

func (k *Klog) Error(fmt string, args ...interface{}) {
	LogError("["+k.prefix+"]: "+fmt, args...)
}

func (k *Klog) Info(fmt string, args ...interface{}) {
	LogInfo("["+k.prefix+"]: "+fmt, args...)
}
func (k *Klog) Debug(fmt string, args ...interface{}) {
	if doDebugLogging {
		LogDebug("["+k.prefix+"]: "+fmt, args...)
	}
}
