package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSyncTimeParser_SetMinutes(t *testing.T) {
	tests := []struct {
		name    string
		minutes string
		wantErr bool
	}{
		// check basic corner cases
		{
			name:    "minutes (59) converted",
			minutes: "59",
			wantErr: false,
		},
		{
			name:    "minutes (00) converted",
			minutes: "00",
			wantErr: false,
		},
		{
			name:    "minutes (-1) not converted",
			minutes: "-1",
			wantErr: true,
		},
		{
			name:    "minutes (100) not converted",
			minutes: "100",
			wantErr: true,
		},
	}

	tmParser := SyncTimeGenerator{}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				err := tmParser.SetMinutes(tt.minutes)
				if tt.wantErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			},
		)
	}
}

func TestSyncTimeParser_SetHours(t *testing.T) {
	tests := []struct {
		name    string
		hours   string
		wantErr bool
	}{
		{
			name:    "hours (23) converted",
			hours:   "23",
			wantErr: false,
		},
		{
			name:    "hours (00) converted",
			hours:   "00",
			wantErr: false,
		},
		{
			name:    "hours (-1) not converted",
			hours:   "-1",
			wantErr: true,
		},
		{
			name:    "hours (24) not converted",
			hours:   "24",
			wantErr: true,
		},
		{
			name:    "hours (100) not converted",
			hours:   "100",
			wantErr: true,
		},
	}

	tmParser := SyncTimeGenerator{}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				err := tmParser.SetHours(tt.hours)
				if tt.wantErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			},
		)
	}
}

func TestSyncTimeParser_SetSeconds(t *testing.T) {
	tests := []struct {
		name    string
		seconds string
		wantErr bool
	}{
		{
			name:    "seconds (59) parsed",
			seconds: "59",
			wantErr: false,
		},
		{
			name:    "seconds (00) parsed",
			seconds: "00",
			wantErr: false,
		},
		{
			name:    "seconds (-1) not parsed",
			seconds: "-1",
			wantErr: true,
		},
		{
			name:    "seconds (60) not parsed",
			seconds: "60",
			wantErr: true,
		},
	}

	tmParser := SyncTimeGenerator{}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				err := tmParser.SetSeconds(tt.seconds)
				if tt.wantErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			},
		)
	}
}

func TestSyncTimeParser_SetupInitialSyncTime(t *testing.T) {
	tests := []struct {
		name    string
		tm      string
		H       int
		M       int
		S       int
		wantErr bool
	}{
		{
			name: "set valid time 1",
			tm:   "00:12:35",
			M:    12,
			S:    35,
		},
		{
			name: "set valid time 2",
			tm:   "23:59:59",
			H:    23,
			M:    59,
			S:    59,
		},
		{
			name: "set valid time 3",
			tm:   "00:00:00",
		},
		{
			name:    "not set invalid time 1",
			tm:      "00:00",
			wantErr: true,
		},
		{
			name:    "not set invalid time 2",
			tm:      "00:00:00:32",
			wantErr: true,
		},
		{
			name:    "not set invalid time 3",
			tm:      "32:00:32",
			wantErr: true,
		},
		{
			name:    "not set invalid time 4",
			tm:      "23:60:32",
			wantErr: true,
		},
		{
			name:    "not set invalid time 5",
			tm:      "23:59:132",
			wantErr: true,
		},
		{
			name:    "not set invalid time 6 (empty string)",
			tm:      "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				tmParser := SyncTimeGenerator{}
				err := tmParser.SetupSyncTime(tt.tm)

				if tt.wantErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
					require.Equal(t, tt.H, tmParser.H)
					require.Equal(t, tt.M, tmParser.M)
					require.Equal(t, tt.S, tmParser.S)
				}
			},
		)
	}
}
