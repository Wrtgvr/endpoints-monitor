package monitor

import (
	"context"
	"log"
	"time"

	"github.com/wrtgvr/websites-monitor/internal/config"
	"github.com/wrtgvr/websites-monitor/internal/domain"
	"github.com/wrtgvr/websites-monitor/internal/storage"
)

var Config *config.MonitorConfig

type Monitor struct {
	Out             chan *domain.EndpointStatus
	storage         storage.EndpointsStorage
	interval        time.Duration
	pingTimeout     time.Duration
	gorutinesAmount int
	ticker          *time.Ticker
}

func NewMonitor(storage storage.EndpointsStorage) *Monitor {
	if Config == nil {
		log.Println("WARN: monitor config is nil")
		return nil
	}
	return &Monitor{
		storage:         storage,
		interval:        Config.Interval,
		pingTimeout:     Config.PingTimeout,
		gorutinesAmount: Config.GorutinesAmount,
	}
}

func (m *Monitor) Run() {
	//* ticker nad out chan
	ticker := time.NewTicker(m.interval)
	m.ticker = ticker

	m.Out = make(chan *domain.EndpointStatus)

	//* ping endpoints every tick
	for range ticker.C {
		endpoints, err := m.storage.GetEndpointsForMonitoring(context.Background())
		if err != nil {
			log.Println("ERR: monitor could not get endpoints")
			continue
		}

		//* semaphore
		sem := semaphore{
			C: make(chan struct{}, m.gorutinesAmount),
		}

		//* ping endpoints
		for _, v := range endpoints {
			go func(ep *domain.EndpointInfo) {
				defer recover()

				sem.acquire()
				defer sem.release()

				//* ping endpoint and send result to monitor output channel
				m.Out <- endpointPing(ep, m.pingTimeout)
			}(v)
		}
	}
}

func (m *Monitor) Stop() {
	m.ticker.Stop()
	close(m.Out)
}
