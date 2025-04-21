// server contains API commands available for user
package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"
	"net/http"
	"sync"
)

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

// LoaderUpdater implements driver behaviour to operate with
// sync source (local file, vault, etc.)
type LoaderUpdater interface {
	LoadSyncConfig() (s SyncConfig, err error)
	UpdateSyncConfig(config SyncConfig) (err error)
}

// Server used for handle API
type Server struct {
	g   *gin.Engine
	b   *Block
	log *logrus.Logger
	cfg *ServerConfig
	d   LoaderUpdater
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

func (srv *Server) SetConfigDriver(d LoaderUpdater) {
	srv.d = d
}

// HandleSyncCommand handle unscheduled used command.
// Return http status 409 if operation running
func (srv *Server) HandleSyncCommand(c *gin.Context) {
	var syncReq SyncDirectoriesRequest
	var err error

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

	// we pass only first query to avoid accident with inconsistent files state
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
	scfg := SyncConfig{
		SrcPath:        syncReq.SrcPath,
		DstPath:        syncReq.DstPath,
		MaxDiffPercent: syncReq.MaxDiffPercent,
	}

	ctx := context.Background()
	if err = SyncDirectories(ctx, srv.log, scfg); err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
	}
}

// UpdateConfiguration command for update server sync configuration
func (srv *Server) UpdateConfiguration(c *gin.Context) {
	var sCfg ChangeSyncConfig
	var err error

	// Validate request
	if err = c.BindJSON(&sCfg); err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	vld := validator.New(validator.WithRequiredStructEnabled())
	if err = vld.Struct(&sCfg); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	syncConfig := SyncConfig{
		SrcPath:        sCfg.SrcPath,
		DstPath:        sCfg.DstPath,
		SyncTime:       sCfg.SyncTime,
		MaxDiffPercent: sCfg.MaxDiffPercent,
	}

	if err = srv.d.UpdateSyncConfig(syncConfig); err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.IndentedJSON(http.StatusOK, 200)
}

// GetCurrentConfig return current synchronization config
func (srv *Server) GetCurrentConfig(c *gin.Context) {
	var cfg SyncConfig
	var err error

	if cfg, err = srv.d.LoadSyncConfig(); err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.IndentedJSON(http.StatusOK, cfg)
}

// Health for lifecycle handling
func (srv *Server) Health(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, 200)
}

// Run server
func (srv *Server) Run(ctx context.Context) (err error) {
	// setup gin router

	if srv.d == nil {
		return fmt.Errorf("config driver not set")
	}

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
		err = server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			srv.log.Error(err)
		}
	}()

	<-ctx.Done()

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

	// setup CORS
	srv.g.Use(
		cors.New(
			cors.Config{
				AllowPrivateNetwork: true,
				AllowHeaders:        srv.cfg.AllowedHeaders,
				AllowMethods:        srv.cfg.AllowedMethods,
				AllowOrigins:        srv.cfg.AllowedHosts,
			},
		),
	)

	// register sync handler
	srv.g.PATCH("/api/v1/sync/directories", srv.HandleSyncCommand)

	// register handler for update server config
	srv.g.PATCH("/api/v1/sync/config/update", srv.UpdateConfiguration)

	// register handler for return actual server config
	srv.g.GET("/api/v1/sync/config", srv.GetCurrentConfig)

	// register /health endpoint for control
	srv.g.GET("/api/v1/health", srv.Health)

	return err
}
