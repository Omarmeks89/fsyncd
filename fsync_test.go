package main

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestSyncCommand_configureSyncActions(t *testing.T) {
	tm := time.Now()

	tests := []struct {
		name        string
		diffPercent int
		srcD        Directory
		dstD        Directory
		wait        []string
	}{
		{
			name:        "test mismatched file will be add to delete",
			diffPercent: 30,
			srcD: Directory{
				Files: map[string]FileMeta{
					"test.txt": {
						ModTime: tm,
					},
				},
			},
			dstD: Directory{
				Files: map[string]FileMeta{
					"any_file.txt": {
						ModTime: tm,
					},
				},
			},
			wait: []string{"./any_file.txt"},
		},
		{
			name:        "test mismatched file will be choose from many",
			diffPercent: 70, // turn difference off
			srcD: Directory{
				Files: map[string]FileMeta{
					"test.txt": {
						ModTime: tm,
					},
				},
			},
			dstD: Directory{
				Files: map[string]FileMeta{
					"test.txt": {
						ModTime: tm,
					},
					"anyfile.txt": {
						ModTime: tm,
					},
				},
			},
			wait: []string{"./anyfile.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				cmd := MakeSyncCommand("", "", tt.diffPercent)
				_ = cmd.configureSyncActions(tt.srcD, tt.dstD)
				require.Equal(t, tt.wait, cmd.ToDelete)
			},
		)
	}
}

func TestSyncCommand_Compare(t *testing.T) {
	tm := time.Now()

	tests := []struct {
		name        string
		diffPercent int
		src         Container
		dest        Container
		wantStatus  bool
		wantErr     error
	}{
		{
			name: "compare equal directories",
			src: &Directory{
				Files: map[string]FileMeta{
					"test.txt": {
						ModTime: tm,
					},
				},
				Root: "/home/user",
			},
			dest: &Directory{
				Files: map[string]FileMeta{
					"test.txt": {
						ModTime: tm,
					},
				},
				Root: "/home/user",
			},
			diffPercent: 30,
			wantStatus:  true,
			wantErr:     nil,
		},
		{
			name: "dest directory have more entities than source",
			src: &Directory{
				Files: map[string]FileMeta{
					"test.txt": {
						ModTime: tm,
					},
				},
				Root: "/home/user",
			},
			dest: &Directory{
				Files: map[string]FileMeta{
					"test.txt": {
						ModTime: tm,
					},
					"new_test.txt": {
						ModTime: tm,
					},
				},
				Root: "/home/user",
			},
			diffPercent: 30,
			wantStatus:  false,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				cmd := MakeSyncCommand("", "", tt.diffPercent)
				status, _ := cmd.Compare(tt.src, tt.dest)

				require.Equal(t, tt.wantStatus, status)
			},
		)
	}
}

func TestSyncCommand_mergeString(t *testing.T) {

	tests := []struct {
		name    string
		str     []string
		wantRes string
	}{
		{
			name:    "test path string concat",
			str:     []string{"/home/user", "/", "data.txt"},
			wantRes: "/home/user/data.txt",
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				cmd := MakeSyncCommand("", "", 0)
				str, _ := cmd.mergePath(tt.str...)
				require.Equal(t, tt.wantRes, str)
			},
		)
	}
}

func TestSyncCommand_configureSyncActions1(t *testing.T) {
	tm := time.Now()

	tests := []struct {
		name string
		src  Directory
		dst  Directory
		res  SyncPair
	}{
		// TODO: Add test cases.
		{
			name: "sync source choose by latest timestamp",
			src: Directory{
				Files: map[string]FileMeta{
					"test.txt": {
						ModTime: tm,
					},
				},
				Root: "/home/user",
			},
			dst: Directory{
				Files: map[string]FileMeta{
					"test.txt": {
						ModTime: tm.Add(10 * time.Second),
					},
				},
				Root: "/cloud/data",
			},
			res: SyncPair{
				Src:      "/cloud/data",
				Dst:      "/home/user",
				FileName: "test.txt",
			},
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				cmd := MakeSyncCommand("", "", 30)
				_ = cmd.configureSyncActions(tt.src, tt.dst)

				// single SyncPair have to equal to sample
				require.Equal(t, tt.res, cmd.SyncPairs[0])
			},
		)
	}
}
