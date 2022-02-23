package main

import (
	"bufio"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var log *zap.SugaredLogger

const (
	namespace = "systemd_resolved"
	resolved_command = "systemd-resolve"
	resolved_args = "--statistics"
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
			if g, ok := metric.(prometheus.Gauge); ok {
				g.Set(v)
			}
			if g, ok := metric.(prometheus.Counter); ok {
				ch <- prometheus.MustNewConstMetric(
					g.Desc(),
					prometheus.CounterValue,
					v)
			}
		}
	}
}

func NewCollector(namespace string) (*Collector, error) {
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
	}, nil
}

func gatherStats() map[string]float64 {

	metrics := make(map[string]float64)

	statusLineRegex := regexp.MustCompile(`[a-zA-Z ]+: ?[0-9]+`)

	cmd := exec.Command(resolved_command, resolved_args)
	stdout, err := cmd.StdoutPipe()

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	cmd.Start()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		l := scanner.Text()
		if statusLineRegex.Match([]byte(l)) {
			//fmt.Println(l)
			f := strings.Split(l, ":")
			k := strings.TrimSpace(f[0])
			v, _ := strconv.ParseFloat(strings.TrimSpace(f[1]), 64)
			log.Debug(k)
			log.Debug(v)
			metrics[k] = v
		}

	}

	cmd.Wait()

	return metrics
}

func main() {
	// set up logger
	cfg := zap.NewDevelopmentConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	cfg.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	logger, _ := cfg.Build()
	log = logger.Sugar()

	collector, _ := NewCollector(namespace)
	prometheus.MustRegister(collector)

	http.Handle("/metrics", promhttp.Handler())

	log.Fatal(http.ListenAndServe(":9924", nil))
}
