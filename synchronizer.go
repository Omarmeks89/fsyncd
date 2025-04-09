// Contains Synchronizer type for handle SyncCommand
package main

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"io"
	"io/fs"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type ItemHandler func(string) error

// Synchronizer for sync command parameters
type Synchronizer struct{}

func MakeSynchronizer() Synchronizer {
	return Synchronizer{}
}

// Sync start sync operation
func (s Synchronizer) Sync(
	ctx context.Context,
	syncCmd SyncCommand,
	log *logrus.Logger,
) (err error) {
	gp := s.CalculatePoolSize()

	// delete directories
	log.WithFields(
		logrus.Fields{
			"stage": "remove_dirs",
			"state": "processing",
		},
	).Debug("deleting directories")
	if err = s.DeleteDirectories(ctx, syncCmd, gp); err != nil {
		return err
	}

	// delete files
	log.WithFields(
		logrus.Fields{
			"stage": "remove_files",
			"state": "processing",
		},
	).Debug("deleting files")
	if err = s.DeleteFiles(ctx, syncCmd, gp); err != nil {
		return err
	}

	// create directories
	log.WithFields(
		logrus.Fields{
			"stage": "create_new_dirs",
			"state": "processing",
		},
	).Debug("creating directories")
	if err = s.CreateDirectories(ctx, syncCmd, gp); err != nil {
		return err
	}

	// sync files
	log.WithFields(
		logrus.Fields{
			"stage": "sync_files",
			"state": "processing",
		},
	).Debug("sync files")
	if err = s.SyncFiles(ctx, log, syncCmd, gp); err != nil {
		return err
	}

	log.WithFields(
		logrus.Fields{
			"stage": "synchronized",
			"state": "success",
		},
	).Debug("synchronization exited")
	return err
}

// DeleteDirectories delete all wished directories from dest concurrently
func (s Synchronizer) DeleteDirectories(
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
func (s Synchronizer) DeleteFiles(
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
func (s Synchronizer) CreateDirectories(
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
func (s Synchronizer) syncPair(
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
	dstFile, err = os.OpenFile(pair.Dst, os.O_CREATE|os.O_RDWR, pair.Perm)
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
func (s Synchronizer) SyncFiles(
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
func (s Synchronizer) deleteFile(file string) (err error) {
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
func (s Synchronizer) deleteDir(dir string) (err error) {
	return os.RemoveAll(dir)
}

// createDirs use MkdirAll under the hood
// Create entire path
func (s Synchronizer) createDirs(
	root string,
	perm fs.FileMode,
) (err error) {
	return os.MkdirAll(root, perm)
}

// CalculatePoolSize for disk io bound tasks
func (s Synchronizer) CalculatePoolSize() int {
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
func (s Synchronizer) handleItems(
	ctx context.Context,
	items []string,
	concurrencyLim int,
	handler ItemHandler,
) (err error) {
	g := errgroup.Group{}

	tokens := make(chan struct{}, concurrencyLim)

	for i := 0; i < concurrencyLim; i++ {
		tokens <- struct{}{}
	}

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

func (s Synchronizer) handleFilePairs(
	ctx context.Context,
	log *logrus.Logger,
	pairs []SyncPair,
	concurrencyLim int,
) (err error) {
	g := errgroup.Group{}

	tokens := make(chan struct{}, concurrencyLim)

	for i := 0; i < concurrencyLim; i++ {
		tokens <- struct{}{}
	}

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

func (s Synchronizer) handleNewDirectories(
	ctx context.Context,
	newDirs []NewDirectory,
	concurrencyLim int,
) (err error) {
	var g errgroup.Group

	tokens := make(chan struct{}, concurrencyLim)

	for i := 0; i < concurrencyLim; i++ {
		tokens <- struct{}{}
	}

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
func (s Synchronizer) fclose(log *logrus.Logger, file io.ReadWriteCloser) {
	if err := file.Close(); err != nil {
		log.Error(err)
	}
}

// SyncByTimer infinite loop activated by timer and run sync operation.
// If b (Block) is locked return error and wait next timer
func SyncByTimer(
	ctx context.Context,
	cfg *ServerConfig,
	b *Block,
) (err error) {

	var tx time.Duration
	var tm time.Time

	// read wished sync time (as '23:12:45' string) and set required
	// hours, minutes and seconds as time.Duration inside
	syncTimeGen := SyncTimeParser{}
	if err = syncTimeGen.SetupInitialSyncTime(cfg.SyncTime); err != nil {
		return err
	}

	// get local time
	// TODO: wrap into top-level operation
	if tm, err = syncTimeGen.GetUTCTime(); err != nil {
		return err
	}

	// truncate by day
	if tx, err = syncTimeGen.SetSyncTime(
		tm,
		tm.Truncate(24*time.Hour),
	); err != nil {
		return err
	}
	// sub current time from wished - we got current sync interval
	t := time.NewTimer(tx)

	// we may use go < 1.23, so we have to care about timers
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			// ...
			return ctx.Err()
		case <-t.C:
			// TODO: wrap into top-level operation
			if b.Lock() {
				// do operation

				if !b.Unlock() {
					panic("broken Block")
				}
			}
			// ---------------------------------------

			// not locked - any other sync running, let`s notify
			// and wait next timer
			// === time (set new interval for next sync)
			// TODO: wrap into top-level operation
			if tm, err = syncTimeGen.GetUTCTime(); err != nil {
				return err
			}
			if tx, err = syncTimeGen.SetSyncTime(
				tm,
				tm.Truncate(24*time.Hour),
			); err != nil {
				return err
			}
			// -----------------------------------------------------------------

			if ok := t.Reset(cfg.GracefulShutdownTimeout); !ok {
				return fmt.Errorf("broken synchronization timer")
			}

			t = time.NewTimer(tx)
		}
	}
}

// DefaultTimePartsSeparator to parse h, m, s
const (
	DefaultTimePartsSeparator = ":"

	// numeric const section

	// RequiredTimePartsCount is 3 for hours, minutes and seconds
	RequiredTimePartsCount = 3

	MinPossibleTimeValue   = 0
	MaxPossibleHoursValue  = 23
	MaxPossibleMinSecValue = 59
)

// SyncTimeParser for handle sync time
type SyncTimeParser struct {
	H time.Duration
	M time.Duration
	S time.Duration
}

// SetupInitialSyncTime convert time string (like 12:45:15) into numeric values
// for hours, minutes and seconds
func (stp *SyncTimeParser) SetupInitialSyncTime(tmFmt string) (err error) {
	if stp == nil {
		return fmt.Errorf("time parser not init")
	}

	parts := strings.Split(tmFmt, DefaultTimePartsSeparator)
	if len(parts) != RequiredTimePartsCount {
		return fmt.Errorf("invalid time parts count")
	}

	if err = stp.SetHours(parts[0]); err != nil {
		return err
	}

	if err = stp.SetMinutes(parts[1]); err != nil {
		return err
	}

	if err = stp.SetSeconds(parts[2]); err != nil {
		return err
	}

	return err
}

func (stp *SyncTimeParser) SetHours(h string) (err error) {
	var hrs int

	if stp == nil {
		return fmt.Errorf("time parser not init")
	}

	if hrs, err = strconv.Atoi(h); err != nil {
		return err
	}

	if hrs < MinPossibleTimeValue || hrs > MaxPossibleHoursValue {
		return fmt.Errorf("hours have to be in between of 0 and 23")
	}

	// set hours as Duration
	stp.H = time.Duration(hrs) * time.Hour
	return err
}

func (stp *SyncTimeParser) SetMinutes(m string) (err error) {
	var mns int

	if stp == nil {
		return fmt.Errorf("time parser not init")
	}

	if mns, err = strconv.Atoi(m); err != nil {
		return err
	}

	if mns < MinPossibleTimeValue || mns > MaxPossibleMinSecValue {
		return fmt.Errorf("minutes have to be in between of 0 and 59")
	}

	// set hours as Duration
	stp.M = time.Duration(mns) * time.Minute
	return err
}

func (stp *SyncTimeParser) SetSeconds(s string) (err error) {
	var sec int

	if stp == nil {
		return fmt.Errorf("time parser not init")
	}

	if sec, err = strconv.Atoi(s); err != nil {
		return err
	}

	if sec < MinPossibleTimeValue || sec > MaxPossibleMinSecValue {
		return fmt.Errorf("seconds have to be in between of 0 and 59")
	}

	// set hours as Duration
	stp.S = time.Duration(sec) * time.Second
	return err
}

func (stp *SyncTimeParser) SetSyncTime(
	origin time.Time,
	truncated time.Time,
) (t time.Duration, err error) {
	if stp == nil {
		return t, fmt.Errorf("time parser not init")
	}

	truncated.Add(stp.H)
	truncated.Add(stp.M)
	truncated.Add(stp.S)

	if origin.After(truncated) {
		// add 24 hours for truncated because it
		// before current time
		truncated.Add(24 * time.Hour)
	}

	return truncated.Sub(origin), err
}

func (stp *SyncTimeParser) GetUTCTime() (t time.Time, err error) {
	if stp == nil {
		return t, fmt.Errorf("time parser not init")
	}
	return time.Now().UTC(), err
}
