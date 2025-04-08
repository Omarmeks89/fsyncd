package main

import (
	"context"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

func main() {
	var cfg = new(ServerConfig)
	var logger *logrus.Logger
	var err error
	var server *Server
	var g errgroup.Group

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g.Go(
		func() error {
			return server.Run(ctx)
		},
	)

	// run synchronization by timer
	g.Go(
		func() error {
			return SyncByTimer(ctx, cfg, block)
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
