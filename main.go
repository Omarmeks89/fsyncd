package main

import (
	"context"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"os/signal"
	"syscall"
)

type ConfigDriver interface {
	LoadSyncConfig() (c SyncConfig, err error)
	UpdateSyncConfig(config SyncConfig) (err error)
}

// default drivers preset
var cfgDrivers = map[string]ConfigDriver{
	DefaultDriver: DefaultConfigDriver{},
}

func main() {
	var cfg = &ServerConfig{}
	var logger *logrus.Logger
	var err error
	var server *Server
	var g errgroup.Group
	var syncCfg SyncConfig

	// load master config for application
	if err = cfg.Load(); err != nil {
		logrus.Fatal(err)
	}

	if logger, err = SetupLogger(cfg.LogLevel, cfg.TimeFormat); err != nil {
		logrus.Fatal(err)
	}

	timeGen := SyncTimeGenerator{}
	if err = timeGen.SetLocalTime(cfg.Location); err != nil {
		logger.Fatal(err)
	}

	logger.Infof("location set: %+v\n", timeGen.location)

	// load sync config
	driver, ok := cfgDrivers[cfg.ConfigDriver]
	if !ok {
		logger.Fatalf("unsupported config driver '%s'\n", cfg.ConfigDriver)
	}

	if syncCfg, err = driver.LoadSyncConfig(); err != nil {
		logger.Fatal(err)
	}

	// create sync operation lock (block)
	block := MakeBlock()
	if server, err = MakeServer(cfg, logger, block); err != nil {
		logger.Fatal(err)
	}

	// make time generator for sync at wished time
	if err = timeGen.SetupSyncTime(syncCfg.SyncTime); err != nil {
		logger.Fatal(err)
	}

	sCtx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	// run sync at start
	if err = SyncDirectories(sCtx, logger, syncCfg); err != nil {
		logger.Error(err)
	}

	syncScheduler, sErr := MakeScheduler(driver, &timeGen)
	if sErr != nil {
		logger.Fatal(err)
	}
	server.SetConfigDriver(driver)

	g.Go(
		func() error {
			return server.Run(sCtx, stop)
		},
	)

	// run synchronization by timer
	g.Go(
		func() error {
			defer stop()
			// we needn`t send stop (cancel) as an argument
			// because we have not gs inside
			return syncScheduler.SyncByTimer(sCtx, logger, block)
		},
	)

	if err = g.Wait(); err != nil {
		logger.Fatal(err)
	}
}
