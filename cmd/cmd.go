package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func TryFatal(log *zap.Logger, err error, msg string, fields ...zap.Field) {
	if err != nil {
		log.Fatal(msg, append(fields, zap.Error(err))...)
	}
}

func RecoverAndLogPanic(log *zap.Logger) {
	if r := recover(); r != nil {
		var err error
		switch x := r.(type) {
		case string:
			err = errors.New(x)
		case error:
			err = x
		default:
			err = fmt.Errorf("unknown panic of type: %T", r)
		}
		log.Error("Panic occurred in serve thread", zap.Error(err))
	}
}

func RunPrometheusHandler(ctx context.Context, log *zap.Logger, address string) {
	h := http.NewServeMux()
	h.Handle("/metrics", promhttp.Handler())
	server := &http.Server{Addr: address, Handler: h}

	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Fatal("Failed to start prometheus handler", zap.Error(err))
		}
	}()

	select {
	case <-ctx.Done():
		log.Info("Shutdown prometheus handler in progress")
		if err := server.Shutdown(ctx); err != nil && err != context.Canceled {
			log.Fatal("Failed to Shutdown prometheus handler", zap.Error(err))
		}
	}
}

func SilentClose(c io.Closer) {
	if c != nil {
		_ = c.Close()
	}
}

func CtxCancelOnOsSignal(log *zap.Logger) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGABRT)

	go func() {
		defer func() {
			signal.Stop(c)
			cancel()
		}()
		select {
		case c2 := <-c:
			log.Info("Received interrupt signal and cancelling context", zap.String("signal", c2.String()))
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx
}

func NewSummaryVector(name string, description string, labels []string) *prometheus.SummaryVec {
	return promauto.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:  "mzg",
		Subsystem:  "mitsproxy",
		Name:       name,
		Help:       description,
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		MaxAge:     1 * time.Hour,
		BufCap:     3 * prometheus.DefBufCap,
	}, labels)
}
