// contain logger presets for daemon

package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"strings"
)

const (
	InfoLevel  = "info"
	DebugLevel = "debug"
	WarnLevel  = "warn"
	ErrorLevel = "error"
	PanicLevel = "panic"
	FatalLevel = "fatal"
)

var ErrUnexpectedLevel = fmt.Errorf("unexpected level")

// SetupLogger return new logger
func SetupLogger(level string, tmFmt string) (log *logrus.Logger, err error) {
	var lv logrus.Level

	log = logrus.New()

	if lv, err = convertLogLevel(level); err != nil {
		return nil, err
	}

	log.SetLevel(lv)
	log.SetFormatter(&logrus.JSONFormatter{TimestampFormat: tmFmt})

	return log, err
}

// convertLogLevel convert log level (as a string) into logrus.Level
func convertLogLevel(level string) (l logrus.Level, err error) {
	level = strings.ToLower(level)

	switch level {
	case InfoLevel:
		return logrus.InfoLevel, err
	case WarnLevel:
		return logrus.WarnLevel, err
	case DebugLevel:
		return logrus.DebugLevel, err
	case ErrorLevel:
		return logrus.ErrorLevel, err
	case PanicLevel:
		return logrus.PanicLevel, err
	case FatalLevel:
		return logrus.FatalLevel, err
	default:
		return logrus.Level(128), ErrUnexpectedLevel
	}
}
