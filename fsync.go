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

	"github.com/sirupsen/logrus"
)

// DefaultBufferSize for intermediate buffer
const DefaultBufferSize = 4096

// DefaultSyncObjectsSize set allocation size for nested SyncCommand collections
const DefaultSyncObjectsSize = 16

const DefaultRdPerm = 0o0644
const DefaultWrPerm = 0o0666

// literal const

// DefaultRootDirMask default name for masked directories
const DefaultRootDirMask = "root"

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

	// Nodes collection to create path inside dst root directory
	ToCreatePath [][]string

	// SyncPairs (src, dst) contain full source and destination paths
	// for synchronized objects
	SyncPairs []SyncPair

	log *logrus.Logger
}

func MakeSyncCommand(log *logrus.Logger, SrcDiffPercent int) SyncCommand {
	toDel := make([]string, 0, DefaultSyncObjectsSize)
	pairs := make([]SyncPair, 0, DefaultSyncObjectsSize)
	paths := make([][]string, 0, DefaultSyncObjectsSize)
	return SyncCommand{
		ToDelete:       toDel,
		SyncPairs:      pairs,
		SrcDiffPercent: SrcDiffPercent,
		ToCreatePath:   paths,
		log:            log,
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
	var dstDirectory Directory

	// check size diff (less than x%) between src and dest directories
	if ok, err = s.CompareRoot(&src, &dst); !ok {
		if err != nil {
			return err
		}

		// domain directories are different - break
		// return signal error
		return TooLargeDifferenceErr
	}

	for dirName, directory := range src.Dirs {
		if dstDirectory, ok = dst.Dirs[dirName]; !ok {

			if dirName == DefaultRootDirMask {
				// opposite root dir not created, we can`t continue
				return fmt.Errorf("no root destination directory")
			}

			// create sync pair for files future dirs
			if err = s.PrepareRootPath(
				dst.MountPoint,
				directory.NestedPath,
			); err != nil {
				return err
			}
		}

		// create task for sync files
		if err = s.configureSyncActions(directory, dstDirectory); err != nil {
			return err
		}
	}

	return err
}

func (s *SyncCommand) PrepareRootPath(
	rootPath string,
	nestedPath string,
) (err error) {
	// root, a, b, c, ..., etc. directories from root/a/b/c/etc. path
	pathChops := make([]string, 0, DefaultSyncObjectsSize)

	chops := strings.Split(nestedPath, "/")
	for _, chop := range chops {

		if strings.Index(chop, ".") > 0 {
			// it`s a file name, not a directory name with leading dot like '.dir'
			// skip
			continue
		}

		// if we got 'root' token - replace on real root path
		chop = strings.Replace(chop, DefaultRootDirMask, rootPath, 1)
		pathChops = append(pathChops, chop)
	}

	s.ToCreatePath = append(s.ToCreatePath, pathChops)
	return err
}

// configureSyncActions generate tasks to sync and tasks to delete
func (s *SyncCommand) configureSyncActions(
	src Directory,
	dst Directory,
) (err error) {
	var srcPath, dstPath, fPath string

	for k, v := range src.Files {
		srcPath, err = s.mergePath(s.prepareRoot(src.NestedPath), "/", k)
		if err != nil {
			return err
		}

		dstPath, err = s.mergePath(s.prepareRoot(dst.NestedPath), "/", k)
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

		fPath, err = s.mergePath(s.prepareRoot(dst.NestedPath), "/", k)
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

// CompareRoot src and dest directory
// return true if entries count are equal or src - dest < x% different
func (s *SyncCommand) CompareRoot(src Sized, dest Sized) (
	status bool,
	err error,
) {
	if s == nil {
		return status, fmt.Errorf("nil receiver not allowed")
	}

	if src == nil || dest == nil {
		return status, fmt.Errorf("nil container not allowed")
	}

	srcSize, dstSize := src.FilesCount(), dest.FilesCount()
	diff := srcSize - dstSize
	if diff < 0 {
		diff = -diff
	}
	maxObj := max(srcSize, dstSize)
	percent := int(float64(diff) / float64(maxObj) * 100)

	// check that diff is less than max possible
	return percent < s.SrcDiffPercent, err
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

// Sized return own size as elements count
type Sized interface {
	FilesCount() int
}

// Directory represent files collection where key is a full path
// and value is modification time
type Directory struct {
	// Files collection of file names (as key) and meta information (as value)
	Files map[string]FileMeta

	// NestedPath path to directory (inside root directory without filename)
	NestedPath string

	// masked directory name in case we
	// need to have same dir names for speedup search
	Mask string

	// current directory real name
	Name string
}

func (dir *Directory) FilesCount() int {
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
	Dirs map[string]Directory

	// MountPoint is equal to root path
	MountPoint string
}

const DefaultDirAllocSize = 16

// MakeSyncMeta factory function return new SyncMeta object
func MakeSyncMeta() SyncMeta {
	dirs := make(map[string]Directory, DefaultDirAllocSize)
	return SyncMeta{
		Dirs: dirs,
	}
}

// MakeMeta iterate through internal root directory objects
// and collect meta information about files
func (sm *SyncMeta) MakeMeta(root string) (err error) {
	// handle nil pointer
	var info os.FileInfo

	if sm == nil {
		return fmt.Errorf("nil SyncMeta pointer")
	}

	if info, err = os.Stat(root); os.IsNotExist(err) {
		return err
	}

	// set mount point
	sm.MountPoint = root

	// build root directory with masked name - later
	// we can find root dirs at sync start by this name
	files := make(map[string]FileMeta, DefaultSyncObjectsSize)
	dir := Directory{
		Mask:       DefaultRootDirMask,
		Name:       info.Name(), // set real name to Name
		NestedPath: root,
		Files:      files,
	}

	sm.Dirs[dir.Mask] = dir
	return sm.makeMeta(root, dir.Mask)
}

// makeMeta do all job
func (sm *SyncMeta) makeMeta(root string, dirName string) (err error) {
	var files []os.DirEntry
	var buf strings.Builder
	var info os.FileInfo

	if files, err = os.ReadDir(root); err != nil {
		return err
	}

	currDir := sm.Dirs[dirName]

	for _, file := range files {
		buf.WriteString(root)
		buf.WriteString("/")
		buf.WriteString(file.Name())

		fPath := buf.String()

		fmt.Printf("%s, %s, %s\n", fPath, root, file.Name())

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

		// create new nested directory
		fCollection := make(map[string]FileMeta, DefaultSyncObjectsSize)
		dir := Directory{
			Mask:       "",
			Name:       file.Name(), // set real name to Name
			NestedPath: fPath,
			Files:      fCollection,
		}

		// save nested directories by real name because
		// they have to be same between synced directories
		// (but root paths are different)
		sm.Dirs[dir.Name] = dir

		// is another directory - dive
		if err = sm.makeMeta(fPath, dir.Name); err != nil {
			return err
		}

		buf.Reset()
	}

	return err
}

// FilesCount return count of files
func (sm *SyncMeta) FilesCount() (size int) {
	for _, directory := range sm.Dirs {
		size += directory.FilesCount()
	}
	return size
}

// fclose internal function for deferred error handling from closed files.
// Can close readers and writers
func fclose(log *logrus.Logger, file io.ReadWriteCloser) {
	if err := file.Close(); err != nil {
		log.Error(err)
	}
}

// Sync files pair
func Sync(ctx context.Context, log *logrus.Logger, pair SyncPair) (err error) {
	var srcFile, dstFile io.ReadWriteCloser

	// open src
	srcFile, err = os.OpenFile(pair.Src, os.O_RDONLY, DefaultRdPerm)
	if err != nil {
		return err
	}

	defer fclose(log, srcFile)

	// open dst (create file if not exists)
	dstFile, err = os.OpenFile(pair.Src, os.O_CREATE|os.O_RDWR, DefaultWrPerm)
	if err != nil {
		return err
	}

	defer fclose(log, dstFile)

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
	_, err = io.CopyBuffer(dstFile, srcFile, buf)
	return err
}
