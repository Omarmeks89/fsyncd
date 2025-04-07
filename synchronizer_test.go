package main

import (
	"testing"
)

func TestSynchronizer_createDirs(t *testing.T) {
	sm, dm, _ := HandlePaths(
		"/home/egor_usual/fsyncd/a",
		"/home/egor_usual/fsyncd/b",
	)

	syncCmd := MakeSyncCommand(50)

	_ = syncCmd.Prepare(sm, dm)

	// start sync
	snc := MakeSynchronizer()
	for _, directory := range syncCmd.DirsToCreate {
		_ = snc.createDirs(directory.DirPath, directory.DirMode)
	}

}
