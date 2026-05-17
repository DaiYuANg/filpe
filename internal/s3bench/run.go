package s3bench

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Run executes the benchmark command and returns a process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	cfg, err := parseConfig(args, stderr)
	if err != nil {
		writeMessage(stderr, err.Error()+"\n")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	runner := bench{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		metrics: newMetrics(),
	}
	if err := runner.run(ctx); err != nil {
		runner.metrics.recordError("fatal: " + err.Error())
	}

	result := runner.metrics.report(cfg)
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		writeMessage(stderr, fmt.Sprintf("encode report: %v\n", err))
		return 1
	}
	if result.Failed > 0 {
		return 1
	}
	return 0
}

func writeMessage(writer io.Writer, message string) {
	if _, err := io.WriteString(writer, message); err != nil {
		panic(err)
	}
}

type bench struct {
	cfg     Config
	client  *http.Client
	metrics *metrics
}

func (b bench) run(ctx context.Context) error {
	if err := b.createBucket(ctx); err != nil {
		return err
	}

	b.runObjectScenario(ctx)
	if !b.cfg.SkipMultipart {
		b.runMultipartScenario(ctx)
	}
	if !b.cfg.SkipErrors {
		b.runErrorScenario(ctx)
	}
	if !b.cfg.KeepObjects {
		b.deleteBucket(ctx)
	}
	return nil
}

func durationMS(value time.Duration) float64 {
	return float64(value.Microseconds()) / 1000
}
