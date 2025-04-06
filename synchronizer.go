// Contains Synchronizer type for handle SyncCommand
package main

import (
	"context"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"io"
	"io/fs"
	"os"
	"runtime"
)

type ItemHandler func(string) error

// Synchronizer for sync command parameters
type Synchronizer struct {
	// wished difference percent between src and dest root directories
	SrcDiffPercent int

	// root path to source directory
	SrcPath string

	// root path to dest directory
	DstPath string
}

// Sync start sync operation
func (s *Synchronizer) Sync(
	ctx context.Context,
	syncCmd SyncCommand,
	log *logrus.Logger,
) (err error) {
	gp := s.CalculatePoolSize()

	// delete directories
	if err = s.DeleteDirectories(ctx, syncCmd, gp); err != nil {
		return err
	}

	// delete files
	if err = s.DeleteFiles(ctx, syncCmd, gp); err != nil {
		return err
	}

	// create directories
	if err = s.CreateDirectories(ctx, syncCmd, gp); err != nil {
		return err
	}

	// sync files
	if err = s.SyncFiles(ctx, log, syncCmd, gp); err != nil {
		return err
	}

	return err
}

// DeleteDirectories delete all wished directories from dest concurrently
func (s *Synchronizer) DeleteDirectories(
	ctx context.Context,
	syncCmd SyncCommand,
	concurrencyLim int,
) (err error) {
	deleteDir := func(str string) error { return s.deleteDir(str) }
	return s.handleItems(
		ctx,
		syncCmd.DirsToDelete,
		concurrencyLim,
		deleteDir,
	)
}

// DeleteFiles delete all wished files from dest concurrently
func (s *Synchronizer) DeleteFiles(
	ctx context.Context,
	syncCmd SyncCommand,
	concurrencyLim int,
) (err error) {
	for _, files := range syncCmd.FilesToDelete {
		funcCall := func(str string) error { return s.deleteFile(str) }
		err = s.handleItems(
			ctx, files, concurrencyLim, funcCall,
		)
		if err != nil {
			return err
		}
	}
	return err
}

// CreateDirectories create all needed directories in dest concurrently
func (s *Synchronizer) CreateDirectories(
	ctx context.Context,
	syncCmd SyncCommand,
	concurrencyLim int,
) (err error) {
	return s.handleNewDirectories(
		ctx,
		syncCmd.DirsToCreate,
		concurrencyLim,
	)
}

// syncPair sync files pair
func (s *Synchronizer) syncPair(
	ctx context.Context,
	log *logrus.Logger,
	pair SyncPair,
) (err error) {
	var srcFile, dstFile io.ReadWriteCloser

	// open src (take permissions from sync pair)
	srcFile, err = os.OpenFile(pair.Src, os.O_RDONLY, pair.Perm)
	if err != nil {
		return err
	}

	defer s.fclose(log, srcFile)

	// open dst (create file if not exists)
	dstFile, err = os.OpenFile(pair.Src, os.O_CREATE|os.O_RDWR, pair.Perm)
	if err != nil {
		return err
	}

	defer s.fclose(log, dstFile)

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

// SyncFiles sync all pairs between source and dest
func (s *Synchronizer) SyncFiles(
	ctx context.Context,
	log *logrus.Logger,
	syncCmd SyncCommand,
	concurrencyLim int,
) (err error) {
	return s.handleFilePairs(ctx, log, syncCmd.SyncPairs, concurrencyLim)
}

// deleteFile delete wished file. If file not exists return nil, if
// any error - error will be type *PathError
//
// Params:
//   - file: entire file path to delete
//
// Returns:
//   - err: if any error returns
func (s *Synchronizer) deleteFile(file string) (err error) {
	if _, err = os.Stat(file); err == nil {
		return os.Remove(file)
	}
	if err != nil && os.IsNotExist(err) {
		// if file not exists - no error
		return nil
	}
	return err
}

// deleteDir use RemoveAll under the hood
func (s *Synchronizer) deleteDir(dir string) (err error) {
	return os.RemoveAll(dir)
}

// createDirs use MkdirAll under the hood
// Create entire path
func (s *Synchronizer) createDirs(
	root string,
	perm fs.FileMode,
) (err error) {
	return os.MkdirAll(root, perm)
}

// CalculatePoolSize for disk io bound tasks
func (s *Synchronizer) CalculatePoolSize() int {
	cc := runtime.NumCPU()
	if cc < 2 {
		return cc
	}

	return cc/2 + 1
}

// HandlePaths handle two paths parallel
func HandlePaths(src string, dst string) (
	srcMeta SyncMeta,
	dstMeta SyncMeta,
	err error,
) {
	var g errgroup.Group

	srcMeta = MakeSyncMeta()
	dstMeta = MakeSyncMeta()

	g.Go(
		func() error {
			return srcMeta.MakeMeta(src)
		},
	)

	g.Go(
		func() error {
			return dstMeta.MakeMeta(dst)
		},
	)

	err = g.Wait()
	return srcMeta, dstMeta, err
}

// handleItems is a concurrent runner that start goroutines pool inside
func (s *Synchronizer) handleItems(
	ctx context.Context,
	items []string,
	concurrencyLim int,
	handler ItemHandler,
) (err error) {
	var g *errgroup.Group

	tokens := make(chan struct{}, concurrencyLim)

	for _, item := range items {
		select {
		case <-ctx.Done():
			goto out
		case <-tokens:
			g.Go(
				func() error {
					err = handler(item)
					tokens <- struct{}{}
					return err
				},
			)
		}
	}

out:
	// wail for all running tasks
	if err = g.Wait(); err != nil {
		return err
	}

	return err
}

func (s *Synchronizer) handleFilePairs(
	ctx context.Context,
	log *logrus.Logger,
	pairs []SyncPair,
	concurrencyLim int,
) (err error) {
	var g *errgroup.Group

	tokens := make(chan struct{}, concurrencyLim)

	for _, pair := range pairs {
		select {
		case <-ctx.Done():
			goto out
		case <-tokens:
			g.Go(
				func() error {
					err = s.syncPair(ctx, log, pair)
					tokens <- struct{}{}
					return err
				},
			)
		}
	}
out:
	if err = g.Wait(); err != nil {
		return err
	}

	return err
}

func (s *Synchronizer) handleNewDirectories(
	ctx context.Context,
	newDirs []NewDirectory,
	concurrencyLim int,
) (err error) {
	var g *errgroup.Group

	tokens := make(chan struct{}, concurrencyLim)

	for _, nd := range newDirs {
		select {
		case <-ctx.Done():
			goto out
		case <-tokens:
			g.Go(
				func() error {
					err = s.createDirs(nd.DirPath, nd.DirMode)
					tokens <- struct{}{}
					return err
				},
			)
		}
	}
out:
	if err = g.Wait(); err != nil {
		return err
	}

	return err
}

// fclose internal function for deferred error handling from closed files.
// Can close readers and writers
func (s *Synchronizer) fclose(log *logrus.Logger, file io.ReadWriteCloser) {
	if err := file.Close(); err != nil {
		log.Error(err)
	}
}
