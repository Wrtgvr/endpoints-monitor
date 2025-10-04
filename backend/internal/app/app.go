package app

import (
	"log"
	"net/http"
	"time"

	"github.com/wrtgvr/websites-monitor/api"
	"github.com/wrtgvr/websites-monitor/internal/config"
	"github.com/wrtgvr/websites-monitor/internal/handlers"
	"github.com/wrtgvr/websites-monitor/internal/monitor"
	"github.com/wrtgvr/websites-monitor/internal/storage"
)

type App struct {
	mux     *http.ServeMux
	storage storage.EndpointsStorage
}

func InitApp(local bool) *App {
	//* storage
	redisCfg := config.GetRedisConfig(true)
	redisStorage := storage.NewRedisStorage(redisCfg)

	//* transport
	responseTimeout := 5 * time.Second

	h := handlers.NewHTTPHandler(redisStorage, responseTimeout)
	mux := http.NewServeMux()

	api.RegisterRoutes(mux, h)

	//* monitor cfg
	monitor.Config = &config.MonitorConfig{
		Interval:        5 * time.Second,
		PingTimeout:     5 * time.Second,
		GorutinesAmount: 3,
	}

	//* app
	return &App{
		mux:     mux,
		storage: redisStorage,
	}
}

func (a *App) Run(addr string) error {
	return http.ListenAndServe(addr, a.mux)
}

func (a *App) MustRun(addr string) {
	if err := a.Run(addr); err != nil {
		log.Fatal(err)
	}
}

func (a *App) Close() error {
	return a.storage.Close()
}
