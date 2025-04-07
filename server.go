// server contains API commands available for user
package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
)

var BrokenServer = fmt.Errorf("broken server")

// Block is used to avoid long time in mutex
type Block struct {
	lock   *sync.RWMutex
	isFree bool
}

func MakeBlock() *Block {
	return &Block{
		lock:   &sync.RWMutex{},
		isFree: true,
	}
}

// IsFree return Block availability for Lock
func (b *Block) IsFree() bool {
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.isFree
}

// Lock try to lock. Return false if Block locked
func (b *Block) Lock() bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	if !b.isFree {
		// lock is locked - return false
		return b.isFree
	}

	b.isFree = false
	return true
}

// Unlock try to unlock Block. Return true if Block unlocked
func (b *Block) Unlock() bool {
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.isFree {
		return b.isFree
	}

	b.isFree = true
	return b.isFree
}

// Server used for handle API
type Server struct {
	g   *gin.Engine
	b   *Block
	log *logrus.Logger
	cfg *ServerConfig
}

// MakeServer factory function for create new server to handle API
func MakeServer(cfg *ServerConfig, log *logrus.Logger, b *Block) (
	s *Server,
	err error,
) {
	if cfg == nil || log == nil || b == nil {
		return s, fmt.Errorf(
			"nil configuration attr: c=%p, l=%p, b=%p",
			cfg,
			log,
			b,
		)
	}

	return &Server{
		b:   b,
		log: log,
		cfg: cfg,
	}, err
}

// HandleSyncCommand handle unscheduled used command.
// Return http status 409 if operation running
func (srv *Server) HandleSyncCommand(c *gin.Context) {
	var syncReq SyncDirectoriesRequest
	var sm, dm SyncMeta
	var err error

	if srv == nil {
		_ = c.AbortWithError(
			http.StatusInternalServerError,
			BrokenServer,
		)
		return
	}

	// Validate request
	if err = c.BindJSON(&syncReq); err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	vld := validator.New(validator.WithRequiredStructEnabled())
	if err = vld.Struct(&syncReq); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if !srv.b.Lock() {
		// sema is closed - return 409 (conflict)
		c.AbortWithStatus(http.StatusConflict)
		return
	}

	defer func() {
		// unlock & panic if Block is not unlocked
		if !srv.b.Unlock() {
			panic("unsafe to continue - broken lock")
		}
	}()

	// we take a lock let`s handle command
	if sm, dm, err = HandlePaths(syncReq.SrcPath, syncReq.DstPath); err != nil {
		_ = c.AbortWithError(
			http.StatusInternalServerError,
			fmt.Errorf("invalid paths"),
		)
		return
	}

	syncCmd := MakeSyncCommand(syncReq.MaxDiffPercent)
	if err = syncCmd.Prepare(sm, dm); err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// start sync
	snc := MakeSynchronizer()
	if err = snc.Sync(context.Background(), syncCmd, srv.log); err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
}

// UpdateConfiguration command for update server sync configuration
func (srv *Server) UpdateConfiguration(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, 200)
}

func (srv *Server) GetCurrentConfig(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, 200)
}

// Run server
func (srv *Server) Run(ctx context.Context) (err error) {
	if srv == nil {
		return BrokenServer
	}

	sCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// setup gin router
	if err = srv.setup(); err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%s", srv.cfg.Host, srv.cfg.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      srv.g,
		ReadTimeout:  srv.cfg.ConnReadTimeout,
		WriteTimeout: srv.cfg.ConnWriteTimeout,
	}

	go func() {
		if err = server.ListenAndServe(); err != nil && !errors.Is(
			err,
			http.ErrServerClosed,
		) {
			srv.log.Error(err)
		}
	}()

	<-sCtx.Done()
	stop()

	srv.log.Debugf("shutting down gracefully, press Ctrl + C to force")

	nc, cancel := context.WithTimeout(
		context.Background(),
		srv.cfg.GracefulShutdownTimeout,
	)
	defer cancel()

	if err = server.Shutdown(nc); err != nil {
		return err
	}

	srv.log.Debugf("server exiting")
	return err
}

func (srv *Server) setup() (err error) {
	// set loglevel for gin
	gin.SetMode(gin.DebugMode)

	srv.g = gin.Default()

	// register sync handler
	srv.g.PATCH("/api/v1/sync/directories", srv.HandleSyncCommand)

	// register handler for update server config
	srv.g.PATCH("/api/v1/server/config/update", srv.UpdateConfiguration)

	// register handler for return actual server config
	srv.g.GET("/api/v1/server/config", srv.GetCurrentConfig)

	return err
}
