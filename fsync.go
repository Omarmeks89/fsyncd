// Module contain functions and types to sync data
// between fs

package main

import (
	"container/list"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
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
var PathError = fmt.Errorf("sequence like '..' not allowed for path")

type SyncPair struct {
	// Src full path to source file
	Src string

	// Dst full path to destination file
	Dst string
}

type DirectoryNode struct {
	Parent *DirectoryNode

	// part of path to directory
	PathPart string
	Nested   map[string]*DirectoryNode

	Visited bool
}

func (dn *DirectoryNode) IsRoot() (ok bool, err error) {
	if dn == nil {
		return ok, fmt.Errorf("nil DirectoryNode receiver not allowed")
	}

	if dn.Parent == nil {
		return true, err
	}

	return ok, err
}

func (dn *DirectoryNode) IsLeaf() (ok bool, err error) {
	if dn == nil {
		return ok, fmt.Errorf("nil DirectoryNode receiver not allowed")
	}

	if dn.Nested == nil || len(dn.Nested) == 0 {
		return true, err
	}

	return ok, err
}

// SyncCommand create all data for successful sync execution
type SyncCommand struct {
	// max possible difference between directories
	SrcDiffPercent int

	// ToDelete contain full paths for files have to be deleted
	// collect in map to run parallel
	ToDelete map[string][]string

	// Nodes collection to create path inside dst root directory
	DirGraph *DirectoryNode

	// SyncPairs (src, dst) contain full source and destination paths
	// for synchronized objects
	SyncPairs []SyncPair

	log *logrus.Logger

	// re expression for detect file suffix like ([a-z...]\\.[a-z]+) group
	suffDetector *regexp.Regexp

	// re expression for .. and ... head sequences
	prefDetector *regexp.Regexp
}

func MakeSyncCommand(log *logrus.Logger, SrcDiffPercent int) SyncCommand {
	toDel := make(map[string][]string, DefaultSyncObjectsSize)
	pairs := make([]SyncPair, 0, DefaultSyncObjectsSize)

	// alloc memory for paths graph
	paths := &DirectoryNode{
		Nested: make(map[string]*DirectoryNode, DefaultSyncObjectsSize),
	}

	sd := regexp.MustCompile("([a-zA-Zа-яА-Я_0-9\\-]+\\.[a-z]+)")
	pd := regexp.MustCompile("^\\.{2,}.*")

	return SyncCommand{
		ToDelete:       toDel,
		SyncPairs:      pairs,
		SrcDiffPercent: SrcDiffPercent,
		DirGraph:       paths,
		log:            log,
		suffDetector:   sd,
		prefDetector:   pd,
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

		// directory not found in dst directory - we have
		// to make task to create this directory in dst
		if dstDirectory, ok = dst.Dirs[dirName]; !ok {

			if dirName == DefaultRootDirMask {
				// opposite root dir not created, we can`t continue
				return fmt.Errorf("no root destination directory")
			}

			if err = s.PrepareRootPath(
				dst.MountPoint,
				directory.NestedPath,
			); err != nil {
				return err
			}
		}

		// overwrite nested path as a full path to directory
		srcFullPath := s.replaceRootMask(
			directory.NestedPath,
			src.MountPoint,
		)

		dstFullPath := s.replaceRootMask(
			directory.NestedPath,
			dst.MountPoint,
		)

		directory.NestedPath = srcFullPath
		dstDirectory.NestedPath = dstFullPath

		// set name (if not set - directory not exists) to
		// group files for delete (if exists)
		if dstDirectory.Name == "" {
			dstDirectory.Name = directory.Name
		}

		// create task for sync files
		if err = s.configureSyncActions(directory, dstDirectory); err != nil {
			return err
		}
	}

	return err
}

// PrepareRootPath prepare path components to create directories
// inside a dst root directory
func (s *SyncCommand) PrepareRootPath(
	rootPath string,
	nestedPath string,
) (err error) {
	var currNode, nested *DirectoryNode
	var ok bool

	// init root node and set path (if not exists)
	currNode = s.DirGraph
	if currNode == nil {
		return fmt.Errorf("directory graph not allocated")
	}

	if currNode.PathPart == "" {
		currNode.PathPart = rootPath
	}

	// root, a, b, c, ..., etc. directories from root/a/b/c/etc. path
	// exclude path with sequences like '..' or longer
	chops := strings.Split(nestedPath, "/")
	if len(chops) < 2 {
		// return error - no valid path to create (root exists)
		return fmt.Errorf("invalid path chops count or no paths al all")
	}

	// exclude root element
	for _, chop := range chops[1:] {

		if s.suffDetector.MatchString(chop) {
			continue
		}

		// if path chop contain prefix like '..' (or more)
		// return path error
		if s.prefDetector.MatchString(chop) {
			return PathError
		}

		newDirNode := &DirectoryNode{
			Parent:   currNode,
			PathPart: chop,
			Nested: make(
				map[string]*DirectoryNode,
				DefaultSyncObjectsSize,
			),
		}

		if nested, ok = currNode.Nested[chop]; !ok {
			// add to Nested in current Node and update pointer
			currNode.Nested[chop] = newDirNode
			currNode = newDirNode
			continue
		}

		// nested node exists - change Parent and add new node to nested
		// update pointer on nested
		newDirNode.Parent = nested
		nested.Nested[chop] = newDirNode
		currNode = nested
	}

	return err
}

// replaceRootMask will replace 'root' mask with exact root path. If
// no 'root' mask found in string - return nestedPath
func (s *SyncCommand) replaceRootMask(
	nestedPath string,
	rootPath string,
) string {
	return strings.Replace(nestedPath, DefaultRootDirMask, rootPath, 1)
}

// configureSyncActions generate tasks to sync and tasks to delete
func (s *SyncCommand) configureSyncActions(
	src Directory,
	dst Directory,
) (err error) {
	var srcPath, dstPath, fPath, delKey string

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

		// make del key
		if delKey, err = s.mergePath(dst.Name, k); err != nil {
			return err
		}

		// add full path to destination
		s.ToDelete[delKey] = append(s.ToDelete[delKey], fPath)
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

		if ok := file.IsDir(); !ok {

			// is a file, let`s add file meta into Directory
			if info, err = file.Info(); err != nil {
				return err
			}

			// save by filename (not by full path)
			currDir.Files[info.Name()] = FileMeta{
				ModTime: info.ModTime(),
			}

			buf.Reset()
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

	// open src (take permissions from sync pair)
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

func DeleteFiles(files []string) (err error) {
	for _, file := range files {

		if err = os.Remove(file); err != nil {
			return err
		}
	}
	return err
}

func CreateDirectoriesBFS(
	root *DirectoryNode,
	perm fs.FileMode,
) (err error) {
	var dq *list.List
	var buf strings.Builder
	var ok bool

	if root == nil {
		return fmt.Errorf("nil pointer on dir root")
	}

	dq = list.New()

	dq.PushBack(root)
	for dq.Len() != 0 {

		// we dont care about nil ptr here
		elem := dq.Front()
		node := elem.Value.(*DirectoryNode)
		dq.Remove(elem)

		for _, directoryNode := range node.Nested {

			buf.WriteString(node.PathPart)
			buf.WriteString("/")
			buf.WriteString(directoryNode.PathPart)

			// create new directory
			if err = os.Mkdir(buf.String(), perm); err != nil {
				return err
			}

			// set new path
			if ok, err = directoryNode.IsLeaf(); err != nil {
				return err
			}

			if !ok {
				// nested dir is not a leaf we should handle
				// it -> update path and add into deque
				directoryNode.PathPart = buf.String()
				dq.PushBack(directoryNode)
			}

			// skip leaf dir
			buf.Reset()
		}

		// we handle all nested nodes - mark current node as visited
		node.Visited = true
	}
	return err
}
