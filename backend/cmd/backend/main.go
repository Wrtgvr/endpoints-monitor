package main

import (
	"flag"

	"github.com/wrtgvr/websites-monitor/internal/app"
)

func main() {
	flag.Parse()

	app := app.InitApp(true)

	app.Run(":8080")
}
