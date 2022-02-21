package main

import (
	"bufio"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var log *zap.SugaredLogger

const (
	namespace = "systemd_resolved"
)

func gatherMetrics() {

	metrics := make(map[string]uint64)

	statusLineRegex := regexp.MustCompile(`[a-zA-Z ]+: ?[0-9]+`)

	cmd :=  exec.Command("systemd-resolve", "--statistics")
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
			v,_ := strconv.ParseUint(strings.TrimSpace(f[1]), 10, 64)
			log.Debug(k)
			log.Debug(v)
			metrics[k] = v
		}

	}
	log.Debug(metrics)

	cmd.Wait()



}

func main() {
	// set up logger
	cfg := zap.NewDevelopmentConfig()
	cfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	cfg.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	logger, _ := cfg.Build()
	log = logger.Sugar()

	gatherMetrics()
}
