package main

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"sort"
	"testing"
	"time"
)

func TestSyncCommand_configureSyncActions(t *testing.T) {
	tm := time.Now()
	log := logrus.New()

	tests := []struct {
		name         string
		diffPercent  int
		srcD         Directory
		dstD         Directory
		waitToDelete map[string][]string
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
				Name: "dir_a",
			},
			dstD: Directory{
				Files: map[string]FileMeta{
					"any_file.txt": {
						ModTime: tm,
					},
				},
				Name: "dir_a",
			},
			waitToDelete: map[string][]string{
				"dir_aany_file.txt": {
					"./any_file.txt",
				},
			},
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
				Name: "dir_b",
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
				Name: "dir_b",
			},
			waitToDelete: map[string][]string{
				"dir_banyfile.txt": {
					"./anyfile.txt",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				cmd := MakeSyncCommand(log, tt.diffPercent)
				_ = cmd.configureSyncActions(tt.srcD, tt.dstD)

				fmt.Printf("%+v, %+v\n", cmd.ToDelete, tt.waitToDelete)

				require.Equal(t, tt.waitToDelete, cmd.ToDelete)
			},
		)
	}
}

func TestSyncCommand_Compare(t *testing.T) {
	tm := time.Now()
	log := logrus.New()

	tests := []struct {
		name        string
		diffPercent int
		src         Sized
		dest        Sized
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
				NestedPath: "/home/user",
			},
			dest: &Directory{
				Files: map[string]FileMeta{
					"test.txt": {
						ModTime: tm,
					},
				},
				NestedPath: "/home/user",
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
				NestedPath: "/home/user",
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
				NestedPath: "/home/user",
			},
			diffPercent: 30,
			wantStatus:  false,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				cmd := MakeSyncCommand(log, tt.diffPercent)
				status, _ := cmd.CompareRoot(tt.src, tt.dest)

				require.Equal(t, tt.wantStatus, status)
			},
		)
	}
}

func TestSyncCommand_mergeString(t *testing.T) {
	log := logrus.New()

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
				cmd := MakeSyncCommand(log, 0)
				str, _ := cmd.mergePath(tt.str...)
				require.Equal(t, tt.wantRes, str)
			},
		)
	}
}

func TestSyncCommand_configureSyncActions1(t *testing.T) {
	tm := time.Now()
	log := logrus.New()

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
				NestedPath: "/home/user",
			},
			dst: Directory{
				Files: map[string]FileMeta{
					"test.txt": {
						ModTime: tm.Add(10 * time.Second),
					},
				},
				NestedPath: "/cloud/data",
			},
			res: SyncPair{
				Src: "/cloud/data/test.txt",
				Dst: "/home/user/test.txt",
			},
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				cmd := MakeSyncCommand(log, 30)
				_ = cmd.configureSyncActions(tt.src, tt.dst)

				// single SyncPair have to equal to sample
				require.Equal(t, tt.res, cmd.SyncPairs[0])
			},
		)
	}
}

func TestSyncCommand_Prepare(t *testing.T) {
	tm := time.Now()
	log := logrus.New()

	tests := []struct {
		name string
		src  SyncMeta
		dst  SyncMeta
		err  error
		res  []SyncPair
	}{
		{
			name: "test create test2.txt in /cloud/sync-dir directory",
			src: SyncMeta{
				Dirs: map[string]Directory{
					"sync-dir": {
						Files: map[string]FileMeta{
							"test1.txt": {
								ModTime: tm,
							},
							"test2.txt": {
								ModTime: tm,
							},
							"test3.txt": {
								ModTime: tm,
							},
						},
						NestedPath: "root/sync-dir",
						Name:       "sync-dir",
					},
				},
				MountPoint: "/home/master",
			},
			dst: SyncMeta{
				Dirs: map[string]Directory{
					"sync-dir": {
						Files: map[string]FileMeta{
							"test1.txt": {
								ModTime: tm,
							},
							"test3.txt": {
								ModTime: tm,
							},
						},
						NestedPath: "root/sync-dir",
						Name:       "sync-dir",
					},
				},
				MountPoint: "/cloud",
			},
			err: nil,
			res: []SyncPair{
				{
					Src: "/home/master/sync-dir/test1.txt",
					Dst: "/cloud/sync-dir/test1.txt",
				},
				{
					Src: "/home/master/sync-dir/test2.txt",
					Dst: "/cloud/sync-dir/test2.txt",
				},
				{
					Src: "/home/master/sync-dir/test3.txt",
					Dst: "/cloud/sync-dir/test3.txt",
				},
			},
		},
		{
			name: "test set test1.txt from /cloud/sync-dir directory as master (latest change)",
			src: SyncMeta{
				Dirs: map[string]Directory{
					"sync-dir": {
						Files: map[string]FileMeta{
							"test1.txt": {
								ModTime: tm,
							},
							"test3.txt": {
								ModTime: tm,
							},
						},
						NestedPath: "root/sync-dir",
						Name:       "sync-dir",
					},
				},
				MountPoint: "/home/master",
			},
			dst: SyncMeta{
				Dirs: map[string]Directory{
					"sync-dir": {
						Files: map[string]FileMeta{
							"test1.txt": {
								ModTime: tm.Add(10 * time.Minute),
							},
							"test3.txt": {
								ModTime: tm,
							},
						},
						NestedPath: "root/sync-dir",
						Name:       "sync-dir",
					},
				},
				MountPoint: "/cloud",
			},
			err: nil,
			res: []SyncPair{
				{
					Src: "/home/master/sync-dir/test3.txt",
					Dst: "/cloud/sync-dir/test3.txt",
				},
				{
					Src: "/cloud/sync-dir/test1.txt",
					Dst: "/home/master/sync-dir/test1.txt",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				cmd := MakeSyncCommand(log, 35)
				_ = cmd.Prepare(tt.src, tt.dst)

				sort.Slice(
					tt.res, func(i, j int) bool {
						return tt.res[i].Src > tt.res[j].Src
					},
				)

				sort.Slice(
					cmd.SyncPairs, func(i, j int) bool {
						return cmd.SyncPairs[i].Src > cmd.SyncPairs[j].Src
					},
				)

				require.EqualExportedValues(t, tt.res, cmd.SyncPairs)
			},
		)
	}
}

func TestSyncCommand_PrepareReturnError(t *testing.T) {
	tm := time.Now()
	log := logrus.New()

	tests := []struct {
		name string
		src  SyncMeta
		dst  SyncMeta
		err  error
	}{
		{
			name: "test return signal error (TooLargeDifferenceErr)",
			src: SyncMeta{
				Dirs: map[string]Directory{
					"sync-dir": {
						Files: map[string]FileMeta{
							"test1.txt": {
								ModTime: tm,
							},
							"test3.txt": {
								ModTime: tm,
							},
						},
						NestedPath: "/home/master/sync-dir",
					},
					"sync-dir2": {
						Files: map[string]FileMeta{
							"test3.txt": {
								ModTime: tm,
							},
							"test4.txt": {
								ModTime: tm,
							},
						},
						NestedPath: "/home/master/sync-dir2",
					},
				},
			},
			dst: SyncMeta{
				Dirs: map[string]Directory{
					"sync-dir": {
						Files: map[string]FileMeta{
							"test1.txt": {
								ModTime: tm,
							},
						},
						NestedPath: "/cloud/sync-dir",
					},
				},
			},
			err: TooLargeDifferenceErr,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				// difference in directories count is 50% - we got error
				cmd := MakeSyncCommand(log, 30)
				err := cmd.Prepare(tt.src, tt.dst)

				require.EqualError(t, err, tt.err.Error())
			},
		)
	}
}

func TestSyncCommand_PrepareRootPath(t *testing.T) {
	var dn *DirectoryNode
	var ok bool

	tests := []struct {
		name        string
		rootPath    string
		nestedPath  string
		createdKeys []string
		res         *DirectoryNode
	}{
		{
			name:        "base path configured",
			rootPath:    "/cloud/root/path",
			nestedPath:  "root/test/my-proj/config.json",
			createdKeys: []string{"test", "my-proj"},
			res: &DirectoryNode{
				Nested: map[string]*DirectoryNode{
					"test": &DirectoryNode{
						Nested: map[string]*DirectoryNode{
							"my-proj": &DirectoryNode{
								PathPart: "my-proj",
							},
						},
						PathPart: "test",
					},
				},
				PathPart: "/cloud/root/path",
			},
		},
		{
			name:        "more complex path configured",
			rootPath:    "/cloud/root/path",
			nestedPath:  "root/test/my-proj/data/upd/.ref.txt",
			createdKeys: []string{"test", "my-proj", "data", "upd"},
			res: &DirectoryNode{
				Nested: map[string]*DirectoryNode{
					"test": &DirectoryNode{
						Nested: map[string]*DirectoryNode{
							"my-proj": &DirectoryNode{
								PathPart: "my-proj",
								Nested: map[string]*DirectoryNode{
									"data": &DirectoryNode{
										PathPart: "data",
										Nested: map[string]*DirectoryNode{
											"upd": &DirectoryNode{
												PathPart: "upd",
											},
										},
									},
								},
							},
						},
						PathPart: "test",
					},
				},
				PathPart: "/cloud/root/path",
			},
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				sm := MakeSyncCommand(logrus.New(), 30)
				_ = sm.PrepareRootPath(tt.rootPath, tt.nestedPath)

				dn = sm.DirGraph

				for i, key := range tt.createdKeys {
					dn, ok = dn.Nested[key]
					require.Equal(t, true, ok)

					if i == len(tt.createdKeys)-1 {
						// check last dn is Leaf
						ok, _ = dn.IsLeaf()
						require.Equal(t, true, ok)
					}
				}
			},
		)
	}
}

func TestSyncCommand_PrepareRootPathReturnPathError(t *testing.T) {
	tests := []struct {
		name       string
		rootPath   string
		nestedPath string
		err        error
	}{
		{
			name:       "invalid path got empty result",
			rootPath:   "/cloud/root/path",
			nestedPath: "root/test/..my-proj/config.json",
			err:        PathError,
		},
		{
			name:       "invalid path got empty result (2)",
			rootPath:   "/cloud/root/path",
			nestedPath: "root/test/../.my-proj/config.json",
			err:        PathError,
		},
		{
			name:       "invalid path got empty result (3)",
			rootPath:   "/cloud/root/path",
			nestedPath: "root/test/........my-proj/config.json",
			err:        PathError,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				sm := MakeSyncCommand(logrus.New(), 30)
				_ = sm.PrepareRootPath(tt.rootPath, tt.nestedPath)

				require.EqualError(t, PathError, tt.err.Error())
			},
		)
	}
}
