package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/godbus/dbus/v5"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"gopkg.in/alecthomas/kingpin.v2"
)

var log *zap.SugaredLogger

const (
	namespace       = "systemd_resolved"
	resolvedCommand = "systemd-resolve"
	resolvedArgs    = "--statistics"
)

type Collector struct {
	namespace    string
	metrics      map[string]prometheus.Collector
	collectMode  string
	gatherDNSSec bool
}

func (c Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, metric := range c.metrics {
		metric.Describe(ch)
	}
}

func (c Collector) Collect(ch chan<- prometheus.Metric) {

	var stats map[string]float64

	switch c.collectMode {
	case "cli":
		stats = gatherStats()
	case "dbus":
		stats = gatherStatsDbus(c.gatherDNSSec)
	default:
		log.Fatal("invalid collect mode:" + c.collectMode)
	}
	log.Debug(stats)

	for k, v := range stats {
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

func NewCollector(namespace string, gatherDNSSec bool, collectMode string) *Collector {
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

	if gatherDNSSec {
		metrics["Secure"] = prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "dnssec_secure_total",
			Help:      "Total number of DNSSEC Verdicts Secure",
		})
		metrics["Insecure"] = prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "dnssec_insecure_total",
			Help:      "Total number of DNSSEC Verdicts Insecure",
		})
		metrics["Bogus"] = prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "dnssec_bogus_total",
			Help:      "Total number of DNSSEC Verdicts Bogus",
		})
		metrics["Indeterminate"] = prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "dnssec_indeterminate_total",
			Help:      "Total number of DNSSEC Verdicts Indeterminate",
		})
	}

	return &Collector{
		namespace:    namespace,
		metrics:      metrics,
		collectMode:  collectMode,
		gatherDNSSec: gatherDNSSec,
	}
}

func gatherStats() map[string]float64 {

	stats := make(map[string]float64)

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
			stats[k] = v
		}
	}

	err = cmd.Wait()
	if err != nil {
		log.Fatal(err)
	}

	return stats
}

func gatherStatsDbus(gatherDNSSec bool) map[string]float64 {
	stats := make(map[string]float64)

	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	obj := conn.Object("org.freedesktop.resolve1", "/org/freedesktop/resolve1")

	cacheStats, err := parseProperty(obj, "org.freedesktop.resolve1.Manager.CacheStatistics")
	if err != nil {
		panic(err)
	}
	stats["Current Cache Size"] = cacheStats[0]
	stats["Cache Hits"] = cacheStats[1]
	stats["Cache Misses"] = cacheStats[2]

	transactionStats, err := parseProperty(obj, "org.freedesktop.resolve1.Manager.TransactionStatistics")
	if err != nil {
		panic(err)
	}
	stats["Current Transactions"] = transactionStats[0]
	stats["Total Transactions"] = transactionStats[1]

	if gatherDNSSec {
		dnssecStats, err := parseProperty(obj, "org.freedesktop.resolve1.Manager.DNSSECStatistics")
		if err != nil {
			panic(err)
		}
		stats["Secure"] = dnssecStats[0]
		stats["Insecure"] = dnssecStats[1]
		stats["Bogus"] = dnssecStats[2]
		stats["Indeterminate"] = dnssecStats[3]
	}

	return stats
}

func parseProperty(object dbus.BusObject, path string) (ret []float64, err error) {
	variant, err := object.GetProperty(path)
	if err != nil {
		return nil, err
	}
	for _, v := range variant.Value().([]interface{}) {
		i := v.(uint64)
		ret = append(ret, float64(i))
	}
	return ret, err
}

func main() {

	var (
		listenAddress = kingpin.Flag("listen-address", "The address to listen on for HTTP requests.").Default(":9924").String()
		debug         = kingpin.Flag("debug", "Debug mode.").Bool()
		gatherDNSSec  = kingpin.Flag("gather-dnssec", "Collect DNSSEC statistics.").Bool()
		collectMode   = kingpin.Flag("collect-mode", "Define how to collect stats. (dbus/cli)").Default("dbus").String()
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

	//log.Debug(gatherStatsDbus())

	collector := NewCollector(namespace, *gatherDNSSec, *collectMode)
	prometheus.MustRegister(collector)

	http.Handle("/metrics", promhttp.Handler())
	log.Info("collect:mode " + *collectMode)
	log.Info("start http handler on " + *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
