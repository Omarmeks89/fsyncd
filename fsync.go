// Module contain functions and types to sync data
// between fs

package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// DefaultBufferSize for intermediate buffer
const DefaultBufferSize = 4096

const DefaultRdPerm = 0o0644
const DefaultWrPerm = 0o0666

// file, err = os.OpenFile(srcPath, os.O_RDONLY, DefaultRdPerm)
// if err != nil {
// return err
// }
// defer Fclose(file)
//
// wFile, err = os.OpenFile(dstPath, os.O_RDWR|os.O_CREATE, DefaultWrPerm)
// if err != nil {
// return err
// }
// defer Fclose(wFile)

// Prepare file in src with dest directory
// func Prepare(reader io.Reader, writer io.Writer) (err error) {
//	var buf = make([]byte, DefaultBufferSize)
//	var n, m int
//
//	_ = io.CopyBuffer()
//
//	for {
//		n, err = reader.Read(buf)
//		if err != nil {
//			if err == io.EOF {
//				err = nil
//			}
//			return err
//		}
//
//		if n == 0 {
//			return fmt.Errorf("no data readed")
//		}
//
//		if m, err = writer.Write(buf[:n]); err != nil {
//			return err
//		}
//
//		if m != n {
//			return fmt.Errorf("write less as readed")
//		}
//	}
// }

type SyncPair struct {
	Src      string
	Dst      string
	FileName string
}

type SyncCommand struct {
	Src string
	Dst string

	// max possible difference between directories
	SrcDiffPercent int

	ToDelete  []string
	SyncPairs []SyncPair

	// buffer to store execution report
	Report strings.Builder
}

func MakeSyncCommand(
	srcPath string,
	dstPath string,
	SrcDiffPercent int,
) SyncCommand {
	toDel := make([]string, 0, 16)
	pairs := make([]SyncPair, 0, 16)
	return SyncCommand{
		Src:            srcPath,
		Dst:            dstPath,
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
		return err
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
		if err = s.configureSyncActions(src.Dirs[i], src.Dirs[i]); err != nil {
			return err
		}
	}

	return err
}

// configureSyncActions generate tasks to sync and tasks to delete
func (s *SyncCommand) configureSyncActions(src Directory, dst Directory) error {
	for k, v := range src.Files {
		if _, ok := dst.Files[k]; !ok {
			// there is no same file in dest directory - skip
			continue
		}

		syncPair := SyncPair{
			Src:      src.Root,
			Dst:      dst.Root,
			FileName: k,
		}

		if v.ModTime.Before(dst.Files[k].ModTime) {
			// rotate roots if file in destination directory
			// have newer version (latest modification time) than
			// file in master directory
			syncPair.Src, syncPair.Dst = syncPair.Dst, syncPair.Src
		}

		delete(dst.Files, k)
		s.SyncPairs = append(s.SyncPairs, syncPair)
	}

	for k, _ := range dst.Files {
		root := dst.Root
		if root == "" {
			root = "."
		}

		fPath, err := s.mergePath(root, "/", k)
		if err != nil {
			return err
		}

		// add full path to destination file
		s.ToDelete = append(s.ToDelete, fPath)
	}

	return nil
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

// Sync prepared directories
func (s *SyncCommand) Sync() (err error) {
	if s == nil {
		return fmt.Errorf("nil receiver not allowed")
	}

	// check prepared data

	// run parallel
	return err
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
	Files map[string]FileMeta
	Root  string
}

func (dir *Directory) Objects() int {
	return len(dir.Files)
}

// FileMeta all required meta data at the moment
type FileMeta struct {
	ModTime time.Time
}

// SyncMeta collect meta information about synchronized
// objects
type SyncMeta struct {
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
