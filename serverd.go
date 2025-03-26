// contain types and functions to work with unix socket

package main

import (
	"context"
	"github.com/sirupsen/logrus"
	"net"
	"time"
)

const UnixSocketNet = "unix"

const DefaultUnixSockTimeout = 10 * time.Second

func sockclose(log *logrus.Logger, sock net.Listener) {
	err := sock.Close()
	if err != nil {
		log.Error(err)
	}
}

// HandleCommands from fsyncdctl
func HandleCommands(
	ctx context.Context,
	log *logrus.Logger,
	sockPath string,
) error {
	// create server unix socket
	servCfg := net.ListenConfig{
		KeepAlive: DefaultUnixSockTimeout, // set timeout for ctl connection
	}

	sock, err := servCfg.Listen(ctx, UnixSocketNet, sockPath)
	if err != nil {
		log.Error(err)
	}

	defer sockclose(log, sock)

	// handle commands
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// more readable
			break
		}

		// handle clients
	}
}
