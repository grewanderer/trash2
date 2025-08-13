package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"wisp/config"
	"wisp/internal/db"
	"wisp/internal/health"
	"wisp/internal/logs"
	"wisp/internal/middleware"
	"wisp/internal/models"
	"wisp/internal/owctrl"
	"wisp/internal/repo"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type App struct {
	cfg        *config.Config
	db         *gorm.DB
	Router     *mux.Router
	httpServer *http.Server

	ctx    context.Context
	cancel context.CancelFunc
}

func (a *App) Initialize(cfg *config.Config) {
	a.cfg = cfg

	/* 1) Логи */
	logs.Init(logs.Options{
		Level:  a.cfg.Logging.Level,
		Format: a.cfg.Logging.Format,
		File:   a.cfg.Logging.File,
	})

	/* 2) DB (опционально) */
	if drv := a.cfg.Database.Driver; drv != "" {
		d, err := db.Open(drv, a.cfg.Database.DSN)
		if err != nil {
			log.Fatalf("db open failed: %v", err)
		}
		a.db = d

		// минимальная доменная модель — только устройство
		if err := a.db.AutoMigrate(&models.Device{}); err != nil {
			log.Fatalf("db migrate failed: %v", err)
		}
	}

	/* 3) Router + middleware */
	a.Router = mux.NewRouter().StrictSlash(true)
	a.Router.Use(
		middleware.RequestID,
		middleware.Recoverer,
		middleware.LoggerMW,
	)

	/* 4) Health */
	if a.db != nil {
		health.RegisterRoutesWithDB(a.Router, a.db) // /healthz, /readyz
	} else {
		health.RegisterRoutes(a.Router) // только /healthz
	}

	/* 5) OpenWISP controller */
	if a.db != nil {
		ds := repo.NewDeviceStore(a.db)
		// Адаптер, реализующий интерфейс owctrl.Store поверх repo.DeviceStore
		a.registerOWRoutesWithStore(newStoreAdapter(ds), a.cfg.OpenWISP.SharedSecret)
	} else {
		owctrl.RegisterRoutes(a.Router, a.cfg.OpenWISP.SharedSecret) // in-memory
	}

	/* (необязательно) вывести известные маршруты в лог при старте */
	_ = a.Router.Walk(func(rt *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		path, _ := rt.GetPathTemplate()
		methods, _ := rt.GetMethods()
		if len(methods) == 0 {
			methods = []string{"ANY"}
		}
		log.Printf("route: %-6v %s", methods, path)
		return nil
	})
}

func (a *App) registerOWRoutesWithStore(store owctrl.Store, secret string) {
	owctrl.RegisterRoutesWithStore(a.Router, secret, store)
}

func (a *App) Run() error {
	if a.Router == nil || a.cfg == nil {
		return fmt.Errorf("server not initialized")
	}

	bind := net.JoinHostPort(a.cfg.Server.Address, a.cfg.Server.HTTPPort)

	a.ctx, a.cancel = context.WithCancel(context.Background())
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		s := <-sigs
		logs.Logger.Infof("shutdown signal: %s", s)
		a.cancel()
	}()

	// Жёсткие таймауты — это важно для production
	a.httpServer = &http.Server{
		Addr:              bind,
		Handler:           a.Router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logs.Logger.Infof("HTTP listening on %s", bind)
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logs.Logger.Fatalf("http server error: %v", err)
		}
	}()

	<-a.ctx.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.httpServer.Shutdown(ctx); err != nil {
		logs.Logger.Errorf("http shutdown: %v", err)
	}
	return nil
}
