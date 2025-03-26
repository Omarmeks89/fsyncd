// Module contain functions and types to sync data
// between fs

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// DefaultBufferSize for intermediate buffer
const DefaultBufferSize = 4096

// DefaultSyncObjectsSize set allocation size for nested SyncCommand collections
const DefaultSyncObjectsSize = 16

const DefaultRdPerm = 0o0644
const DefaultWrPerm = 0o0666

var TooLargeDifferenceErr = fmt.Errorf("too many files not exists")

type SyncPair struct {
	// Src full path to source file
	Src string

	// Dst full path to destination file
	Dst string
}

type SyncCommand struct {
	// max possible difference between directories
	SrcDiffPercent int

	// ToDelete contain full paths for files have to be deleted
	ToDelete []string

	// SyncPairs (src, dst) contain full source and destination paths
	// for synchronized objects
	SyncPairs []SyncPair

	// buffer to store execution report
	Report strings.Builder
}

func MakeSyncCommand(SrcDiffPercent int) SyncCommand {
	toDel := make([]string, 0, DefaultSyncObjectsSize)
	pairs := make([]SyncPair, 0, DefaultSyncObjectsSize)
	return SyncCommand{
		ToDelete:       toDel,
		SyncPairs:      pairs,
		SrcDiffPercent: SrcDiffPercent,
	}
}

// Prepare meta information for synchronization
// Return error if sync is impossible
func (s *SyncCommand) Prepare(src SyncMeta, dst SyncMeta) (err error) {
	if s == nil {
		return fmt.Errorf("nil receiver not allowed")
	}
	return s.prepare(src, dst)
}

// prepare nested do all work
func (s *SyncCommand) prepare(src SyncMeta, dst SyncMeta) (err error) {
	var ok bool

	// check size diff (less than x%) between src and dest
	if ok, err = s.Compare(&src, &dst); !ok {
		// domain directories are different - break
		// return signal error
		return TooLargeDifferenceErr
	}

	// compare fetched metadata - check size diff
	for i := 0; i < src.Objects(); i++ {
		if ok, err = s.Compare(&src.Dirs[i], &dst.Dirs[i]); !ok {
			// means directories are different (diff is greater that x%)
			// create notification for user
			s.makeReport()
			continue
		}

		// add objects for delete (not in master) -> Delete()
		if err = s.configureSyncActions(src.Dirs[i], dst.Dirs[i]); err != nil {
			return err
		}
	}

	return err
}

// configureSyncActions generate tasks to sync and tasks to delete
func (s *SyncCommand) configureSyncActions(
	src Directory,
	dst Directory,
) (err error) {
	var srcPath, dstPath, fPath string

	for k, v := range src.Files {
		srcPath, err = s.mergePath(s.prepareRoot(src.Root), "/", k)
		if err != nil {
			return err
		}

		dstPath, err = s.mergePath(s.prepareRoot(dst.Root), "/", k)
		if err != nil {
			return err
		}

		syncPair := SyncPair{
			Src: srcPath,
			Dst: dstPath,
		}

		// if file by key not exists we will handle empty time value
		if v.ModTime.Before(dst.Files[k].ModTime) {
			// rotate roots if file in destination directory
			// have newer version (latest modification time) than
			// file in master directory
			syncPair.Src, syncPair.Dst = syncPair.Dst, syncPair.Src
		}

		if _, ok := dst.Files[k]; ok {
			delete(dst.Files, k)
		}

		s.SyncPairs = append(s.SyncPairs, syncPair)
	}

	for k, _ := range dst.Files {

		fPath, err = s.mergePath(s.prepareRoot(dst.Root), "/", k)
		if err != nil {
			return err
		}

		// add full path to destination file
		s.ToDelete = append(s.ToDelete, fPath)
	}

	return nil
}

func (s *SyncCommand) prepareRoot(root string) string {
	if root == "" {
		return "."
	}
	return root
}

// Compare src and dest directory
// return true if entries count are equal or src - dest < x% different
func (s *SyncCommand) Compare(src Container, dest Container) (
	status bool,
	err error,
) {
	if s == nil {
		return status, fmt.Errorf("nil receiver not allowed")
	}

	if src == nil || dest == nil {
		return status, fmt.Errorf("nil container not allowed")
	}

	diff := src.Objects() - dest.Objects()
	if diff < 0 {
		diff = -diff
	}
	maxObj := max(src.Objects(), dest.Objects())
	percent := int(float64(diff) / float64(maxObj) * 100)

	// check that diff is less than max possible
	return percent < s.SrcDiffPercent, err
}

func (s *SyncCommand) makeReport() {

}

func (s *SyncCommand) mergePath(str ...string) (res string, err error) {
	var buf strings.Builder

	for _, sp := range str {
		if _, err = buf.WriteString(sp); err != nil {
			return res, err
		}
	}
	return buf.String(), err
}

// Container return own size as elements count
type Container interface {
	Objects() int
}

// Directory represent files collection where key is a full path
// and value is modification time
type Directory struct {
	// Files collection of file names (as key) and meta information (as value)
	Files map[string]FileMeta

	// Root path to directory (without filename)
	Root string
}

func (dir *Directory) Objects() int {
	return len(dir.Files)
}

// FileMeta all required meta data at the moment
type FileMeta struct {
	// ModTime contain last modification time
	ModTime time.Time
}

// SyncMeta collect meta information about synchronized
// objects
type SyncMeta struct {
	// slice of directories for sync
	Dirs []Directory
}

const DefaultDirAllocSize = 16

// MakeSyncMeta factory function return new SyncMeta object
func MakeSyncMeta() SyncMeta {
	dirs := make([]Directory, 0, DefaultDirAllocSize)
	return SyncMeta{
		Dirs: dirs,
	}
}

// MakeMeta iterate through internal root directory objects
// and collect meta information about files
func (sm *SyncMeta) MakeMeta(root string) (err error) {
	var files []os.DirEntry
	var buf strings.Builder
	var info os.FileInfo

	// handle nil pointer
	if sm == nil {
		return fmt.Errorf("nil SyncMeta pointer")
	}

	if files, err = os.ReadDir(root); err != nil {
		return err
	}

	currDir := Directory{}
	for _, file := range files {
		buf.WriteString(root)
		buf.WriteString("/")
		buf.WriteString(file.Name())

		fPath := buf.String()
		if ok := file.IsDir(); !ok {

			// is a file, let`s add file meta into Directory
			if info, err = file.Info(); err != nil {
				return err
			}

			// save by filename (not by full path)
			currDir.Files[info.Name()] = FileMeta{
				ModTime: info.ModTime(),
			}
			continue
		}

		// is another directory - dive
		if err = sm.MakeMeta(fPath); err != nil {
			return err
		}
	}

	// add visited dir to collection
	sm.Dirs = append(sm.Dirs, currDir)
	return err
}

// Objects return count of nested directories
func (sm *SyncMeta) Objects() int {
	return len(sm.Dirs)
}

// Sync files pair
func Sync(ctx context.Context, pair SyncPair) (err error) {
	var srcFile io.ReadCloser
	var dstFile io.WriteCloser

	// open src
	srcFile, err = os.OpenFile(pair.Src, os.O_RDONLY, DefaultRdPerm)
	if err != nil {
		return err
	}

	defer srcFile.Close()

	// open dst (create file if not exists)
	dstFile, err = os.OpenFile(pair.Src, os.O_CREATE|os.O_RDWR, DefaultWrPerm)
	if err != nil {
		return err
	}

	defer dstFile.Close()

	// handle ctx or signal (graceful shutdown)
	// later we can`t stop operation - it may break file...
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		break
	}

	// alloc buffer if files opened
	buf := make([]byte, DefaultBufferSize)
	if _, err = io.CopyBuffer(dstFile, srcFile, buf); err != nil {
		return err
	}

	return err
}
