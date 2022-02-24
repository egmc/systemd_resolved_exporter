package main

import (
	"bufio"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"gopkg.in/alecthomas/kingpin.v2"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var log *zap.SugaredLogger

const (
	namespace       = "systemd_resolved"
	resolvedCommand = "systemd-resolve"
	resolvedArgs    = "--statistics"
)

type Collector struct {
	namespace string
	metrics   map[string]prometheus.Collector
}

func (c Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, metric := range c.metrics {
		metric.Describe(ch)
	}
}

func (c Collector) Collect(ch chan<- prometheus.Metric) {

	for k, v := range gatherStats() {
		if metric, exist := c.metrics[k]; exist {

			switch m := metric.(type) {
			case prometheus.Gauge:
				ch <- prometheus.MustNewConstMetric(
					m.Desc(),
					prometheus.GaugeValue,
					v)
			case prometheus.Counter:
				ch <- prometheus.MustNewConstMetric(
					m.Desc(),
					prometheus.CounterValue,
					v)
			default:
				log.Fatal("invalid metric type")
			}
		}
	}
}

func NewCollector(namespace string) *Collector {
	metrics := make(map[string]prometheus.Collector)

	metrics["Current Transactions"] = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "current_transactions",
		Help:      "Current Transactions",
	})
	metrics["Total Transactions"] = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "transactions_total",
		Help:      "Total Transactions",
	})
	metrics["Current Cache Size"] = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "current_cache_size",
		Help:      "Current Cache Size",
	})
	metrics["Cache Hits"] = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "cache_hits_total",
		Help:      "Total Cache Hits",
	})
	metrics["Cache Misses"] = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "cache_misses_total",
		Help:      "Total Cache Misses",
	})

	return &Collector{
		namespace: namespace,
		metrics:   metrics,
	}
}

func gatherStats() map[string]float64 {

	metrics := make(map[string]float64)

	statusLineRegex := regexp.MustCompile(`[a-zA-Z ]+: ?[0-9]+`)

	cmd := exec.Command(resolvedCommand, resolvedArgs)
	stdout, err := cmd.StdoutPipe()

	if err != nil {
		log.Fatal(err)
	}

	err = cmd.Start()
	if err != nil {
		log.Fatal(err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		l := scanner.Text()
		if statusLineRegex.Match([]byte(l)) {
			f := strings.Split(l, ":")
			k := strings.TrimSpace(f[0])
			v, _ := strconv.ParseFloat(strings.TrimSpace(f[1]), 64)
			metrics[k] = v
		}
	}

	err = cmd.Wait()
	if err != nil {
		log.Fatal(err)
	}

	log.Debug(metrics)

	return metrics
}

func main() {

	var (
		listenAddress = kingpin.Flag("listen-address", "The address to listen on for HTTP requests.").Default(":9924").String()
		debug         = kingpin.Flag("debug", "Debug mode.").Bool()
	)

	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	// set up logger
	logger, _ := zap.NewProduction()
	if *debug {
		logger, _ = zap.NewDevelopment()
	}
	defer func() { err := logger.Sync(); fmt.Printf("Error: %v\n", err) }()
	log = logger.Sugar()

	collector := NewCollector(namespace)
	prometheus.MustRegister(collector)

	http.Handle("/metrics", promhttp.Handler())
	log.Info("start http handler on " + *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
