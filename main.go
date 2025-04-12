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
	"default": DefaultConfigDriver{},
}

func main() {
	var cfg = new(ServerConfig)
	var logger *logrus.Logger
	var err error
	var server *Server
	var g errgroup.Group
	var syncCfg SyncConfig

	timeGen := SyncTimeGenerator{}
	if err = timeGen.SetLocalTime(); err != nil {
		logrus.WithFields(
			logrus.Fields{
				"stage": "set_locale",
				"state": "failed",
				"error": err.Error(),
			},
		).Fatal(err)
	}

	logrus.WithFields(
		logrus.Fields{
			"stage": "init",
			"state": "processing",
		},
	).Infof("location set: %+v\n", timeGen.location)

	// load master config for application
	if err = cfg.Load(); err != nil {
		logrus.WithFields(
			logrus.Fields{
				"stage": "load_config",
				"state": "failed",
				"error": err.Error(),
			},
		).Fatal(err)
	}

	if logger, err = SetupLogger(cfg.LogLevel, cfg.TimeFormat); err != nil {
		logrus.WithFields(
			logrus.Fields{
				"stage": "setup_logger",
				"state": "failed",
				"error": err.Error(),
			},
		).Fatal(err)
	}

	// load sync config
	driver, ok := cfgDrivers[cfg.ConfigDriver]
	if !ok {
		logrus.WithFields(
			logrus.Fields{
				"stage": "setup_sync_driver",
				"state": "failed",
				"error": err.Error(),
			},
		).Fatalf("unsupported config driver '%s'\n", cfg.ConfigDriver)
	}

	if syncCfg, err = driver.LoadSyncConfig(); err != nil {
		logrus.WithFields(
			logrus.Fields{
				"stage": "setup_sync_settings",
				"state": "failed",
				"error": err.Error(),
			},
		).Fatal(err)
	}

	// create sync operation lock (block)
	block := MakeBlock()
	if server, err = MakeServer(cfg, logger, block); err != nil {
		logrus.WithFields(
			logrus.Fields{
				"stage": "setup_server",
				"state": "failed",
				"error": err.Error(),
			},
		).Fatal(err)
	}

	// make time generator for sync at wished time
	if err = timeGen.SetupSyncTime(syncCfg.SyncTime); err != nil {
		logrus.WithFields(
			logrus.Fields{
				"stage":    "setup_sync_interval",
				"state":    "failed",
				"interval": syncCfg.SyncTime,
				"error":    err.Error(),
			},
		).Fatal(err)
	}

	sCtx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	// run sync at start
	if err = SyncDirectories(sCtx, logger, syncCfg); err != nil {
		logrus.WithFields(
			logrus.Fields{
				"stage":    "sync_at_start",
				"state":    "failed",
				"interval": syncCfg.SyncTime,
				"error":    err.Error(),
			},
		).Error(err)
	}

	syncScheduler, sErr := MakeScheduler(driver, &timeGen)
	if sErr != nil {
		logrus.WithFields(
			logrus.Fields{
				"stage": "run_server",
				"state": "failed",
				"error": err.Error(),
			},
		).Fatal(err)
	}
	server.SetConfigDriver(driver)

	g.Go(
		func() error {
			e := server.Run(sCtx)
			stop()
			return e
		},
	)

	// run synchronization by timer
	g.Go(
		func() error {
			e := syncScheduler.SyncByTimer(sCtx, logger, block)
			stop()
			return e
		},
	)

	if err = g.Wait(); err != nil {
		logrus.WithFields(
			logrus.Fields{
				"stage": "run_server",
				"state": "failed",
				"error": err.Error(),
			},
		).Fatal(err)
	}
}
