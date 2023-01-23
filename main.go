package main

import (
	"flag"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"os"
)

const VERSION = `0.3.1`

var (
	ErrorLog         = log.New(os.Stderr, `error#`, log.Lshortfile)
	DebugLog         = log.New(os.Stdout, `debug#`, log.Lshortfile)
	PrometheusErrors = prometheus.NewCounterVec(prometheus.CounterOpts{Name: `errors`,
		Help: `Errors counter`}, []string{`action`, `get`, `repost`})
)

func helpText() {
	fmt.Println(`# https://github.com/vvampirius/get-and-repost`)
	flag.PrintDefaults()
}

func Pong(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, `PONG`)
}

func main() {
	help := flag.Bool("h", false, "print this help")
	ver := flag.Bool("v", false, "Show version")
	configFilePath := flag.String("c", "config.yml", "Path to YAML config")
	flag.Parse()

	if *help {
		helpText()
		os.Exit(0)
	}

	if *ver {
		fmt.Println(VERSION)
		os.Exit(0)
	}

	fmt.Printf("Starting version %s...\n", VERSION)

	if err := prometheus.Register(PrometheusErrors); err != nil {
		ErrorLog.Println(err.Error())
		os.Exit(1)
	}

	configFile, err := NewConfigFile(*configFilePath)
	if err != nil {
		os.Exit(1)
	}

	if _, err = NewCore(configFile); err != nil {
		os.Exit(1)
	}

	server := http.Server{Addr: configFile.Config.Listen}
	http.HandleFunc(`/ping`, Pong)
	http.Handle("/metrics", promhttp.Handler())
	if err := server.ListenAndServe(); err != nil {
		ErrorLog.Fatalln(err.Error())
	}

}
