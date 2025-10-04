package config

import (
	"flag"
	"time"
)

type MonitorConfig struct {
	Interval        time.Duration
	PingTimeout     time.Duration
	GorutinesAmount int
}

var (
	// flags
	flagMonitorInterval = flag.Int64("monitor_interval", 60, "interval (in sec) between endpoints pings")
)

func GetMonitorConfig() *MonitorConfig {
	return &MonitorConfig{
		Interval: getMonitorInterval(),
	}
}

func getMonitorInterval() time.Duration {
	return time.Duration(*flagMonitorInterval) * time.Second
}
