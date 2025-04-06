package main

import (
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_convertLogLevel(t *testing.T) {

	tests := []struct {
		name    string
		level   string
		wantL   logrus.Level
		wantErr bool
		err     error
	}{
		{
			name:  "test info level upper case",
			level: "INFO",
			wantL: logrus.InfoLevel,
			err:   nil,
		},
		{
			name:  "test debug level lower case",
			level: "debug",
			wantL: logrus.DebugLevel,
			err:   nil,
		},
		{
			name:  "test warn level",
			level: "warn",
			wantL: logrus.WarnLevel,
			err:   nil,
		},
		{
			name:  "test error level upper case",
			level: "ERROR",
			wantL: logrus.ErrorLevel,
			err:   nil,
		},
		{
			name:  "test panic level",
			level: "panic",
			wantL: logrus.PanicLevel,
			err:   nil,
		},
		{
			name:  "test fatal level",
			level: "fatal",
			wantL: logrus.FatalLevel,
			err:   nil,
		},
		{
			name:    "test unexpected leve error",
			level:   "ANY",
			wantL:   logrus.Level(128),
			wantErr: true,
			err:     UnexpectedLevel,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				l, err := convertLogLevel(tt.level)

				require.Equal(t, tt.wantL, l)

				if tt.wantErr {
					require.EqualError(t, err, tt.err.Error())
				}
			},
		)
	}
}
