//
//  Copyright (c) 2017, Stardog Union. <http://stardog.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package broker

import (
	"fmt"
	"log"
	"strings"
)

const (
	// ERROR level will only log error messages.
	ERROR = 1
	// WARN level will log warnings and error messages.
	WARN = 2
	// INFO level will log information, warnings, and errors.
	INFO = 3
	// DEBUG level will log all messages and produces a large amount of detail.
	DEBUG = 4
)

var (
	// This map is setup to quickly translate from log level to string at log time
	debugToStringMap = make(map[int]string)
	logLevelNames    []string
)

func init() {
	debugToStringMap[DEBUG] = "DEBUG"
	debugToStringMap[INFO] = "INFO"
	debugToStringMap[WARN] = "WARN"
	debugToStringMap[ERROR] = "ERROR"

	logLevelNames = make([]string, len(debugToStringMap), len(debugToStringMap))
	i := 0
	for _, v := range debugToStringMap {
		logLevelNames[i] = v
		i = i + 1
	}
}

// SdLogger is the stardog logger for Go.  It wraps up logging in a convenient way.
type SdLogger interface {
	Logf(level int, format string, v ...interface{})
}

type sdLogger struct {
	logLevel int
	logger   *log.Logger
}

// NewSdLogger creates a new SdLogger based on the passed in Go standard logger.
func NewSdLogger(realLogger *log.Logger, logLevel string) (SdLogger, error) {
	var logger sdLogger
	logLevel = strings.ToUpper(logLevel)

	switch logLevel {
	case "DEBUG":
		logger.logLevel = DEBUG
	case "INFO":
		logger.logLevel = INFO
	case "WARN":
		logger.logLevel = WARN
	case "ERROR":
		logger.logLevel = ERROR
	default:
		return nil, fmt.Errorf("The log level must be one of DEBUG, INFO, WARN, or ERROR")
	}
	logger.logger = realLogger

	return &logger, nil
}

func (l *sdLogger) logit(lineLevel int, format string, v ...interface{}) {
	if lineLevel > l.logLevel {
		return
	}
	format = fmt.Sprintf("[%s] ", debugToStringMap[lineLevel]) + format
	l.logger.Printf(format, v...)
	l.logger.Println("")
}

// Logf sends the formatted message to the logger at level.
func (l *sdLogger) Logf(level int, format string, v ...interface{}) {
	l.logit(level, format, v...)
}
