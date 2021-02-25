package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

const (
	defaultTimeout           = 180 * time.Second
	defaultPrometheusAddress = ":9200"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	flagInsecureSkipVerify := flag.Bool("insecure-skip-verify", false, "allow insecure skipping of peer verification, when talking to the server")
	flagLocalAddr := flag.String("addr", "0.0.0.0:8080", "address to listen to like 0.0.0.0:8001")
	flagRemoteAddr := flag.String("server", "", "address of the tls socks server like 0.0.0.0:8000")
	flag.Parse()

	log.Info(
		"Starting socks proxy to listen on addr and forward requests to server",
		zap.String("local_addr", *flagLocalAddr),
		zap.String("remote_addr", *flagRemoteAddr),
	)

	localListener, err := net.Listen("tcp", *flagLocalAddr)
	if err != nil {
		log.Fatal("Error listening for incoming socks connections", zap.Error(err))
	}

	defer silentClose(localListener)

	tlsConfig := &tls.Config{
		InsecureSkipVerify: *flagInsecureSkipVerify,
	}
	if tlsConfig.InsecureSkipVerify {
		log.Warn("Running without verification of the tls server - this is dangerous")
	}
	ctx := ctxCancelOnOsSignal(log)

	go runPrometheusHandler(ctx, log, defaultPrometheusAddress)

	var connID uint64
	for {
		localConn, err := localListener.Accept()
		if err != nil {
			log.Fatal("error accepting incoming connections", zap.Error(err))
		}
		connID++
		go serve(ctx, log, localConn, *flagRemoteAddr, tlsConfig, connID)
	}
}

func serve(ctx context.Context, logger *zap.Logger, localConn net.Conn, remoteAddress string, tlsConfig *tls.Config, connID uint64) {
	start := time.Now()

	// Recover if a panic occurs
	defer recoverAndLogPanic(logger)
	defer silentClose(localConn)

	remoteConn, err := tls.DialWithDialer(&net.Dialer{
		Timeout: defaultTimeout,
	}, "tcp", remoteAddress, tlsConfig)
	if err != nil {
		logger.Warn("could not reach remote tls server", zap.Error(err))
		return
	}
	defer silentClose(remoteConn)

	p := &proxy{
		log:   logger,
		wait:  make(chan struct{}),
		lconn: localConn,
	}

	go p.pipe(ctx, remoteConn, localConn)
	go p.pipe(ctx, localConn, remoteConn)

	<-p.wait
	logger.Info(
		"request served",
		zap.Duration("duration", time.Now().Sub(start)),
		zap.Uint64("bytes_sent", atomic.LoadUint64(&p.sentBytes)),
		zap.Uint64("bytes_received", atomic.LoadUint64(&p.receivedBytes)),
		zap.Uint64("conn_id", connID),
		zap.String("from", localConn.RemoteAddr().String()),
	)
}

type proxy struct {
	log           *zap.Logger
	sentBytes     uint64
	receivedBytes uint64
	lconn         io.ReadWriteCloser
	wait          chan struct{}
	erredMU       sync.Mutex
	erred         uint32
}

func (p *proxy) pipe(ctx context.Context, dst io.Writer, src io.Reader) {
	isLocal := src == p.lconn
	buff := make([]byte, 65535)
	for {
		if ctx.Err() != nil {
			p.err("context error", ctx.Err())
			return
		}

		n, err := src.Read(buff[:])
		if err != nil {
			p.err("Read failed", err)
			return
		}

		n, err = dst.Write(buff[:n])
		if err != nil {
			p.err("Write failed", err)
			return
		}
		if isLocal {
			atomic.AddUint64(&p.sentBytes, uint64(n))
		} else {
			atomic.AddUint64(&p.receivedBytes, uint64(n))
		}
	}
}

func (p *proxy) err(message string, err error) {
	if atomic.LoadUint32(&p.erred) == 1 {
		return
	}
	if err != io.EOF {
		p.log.Warn(message, zap.Error(err))
	}

	p.wait <- struct{}{}
	atomic.StoreUint32(&p.erred, 1)
	close(p.wait)
}

func recoverAndLogPanic(logger *zap.Logger) {
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
		logger.Error("Panic occurred in serve thread", zap.Error(err))
	}
}

func runPrometheusHandler(ctx context.Context, logger *zap.Logger, address string) {
	h := http.NewServeMux()
	h.Handle("/metrics", promhttp.Handler())
	server := &http.Server{Addr: address, Handler: h}

	go func() {
		if err := server.ListenAndServe(); err != nil {
			logger.Fatal("Failed to start prometheus handler", zap.Error(err))
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("Shutdown prometheus handler in progress")
		if err := server.Shutdown(ctx); err != nil && err != context.Canceled {
			logger.Fatal("Failed to Shutdown prometheus handler", zap.Error(err))
		}
	}
}

func silentClose(closer io.Closer) {
	_ = closer.Close()
}

func ctxCancelOnOsSignal(logger *zap.Logger) context.Context {
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
			logger.Info("Received interrupt signal and cancelling context", zap.String("signal", c2.String()))
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx
}
