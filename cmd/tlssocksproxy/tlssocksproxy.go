package main

import (
	"context"
	"crypto/tls"
	"flag"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/foomo/tlssocks/cmd"
	"go.uber.org/zap"
)

const (
	defaultTimeout           = 180 * time.Second
	defaultPrometheusAddress = ":9200"
	connDeadline             = 60 * time.Second
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

	defer cmd.SilentClose(localListener)

	tlsConfig := &tls.Config{
		InsecureSkipVerify: *flagInsecureSkipVerify,
	}
	if tlsConfig.InsecureSkipVerify {
		log.Warn("Running without verification of the tls server - this is dangerous")
	}
	ctx := cmd.CtxCancelOnOsSignal(log)

	go cmd.RunPrometheusHandler(ctx, log, defaultPrometheusAddress)

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
	defer cmd.RecoverAndLogPanic(logger)
	defer cmd.SilentClose(localConn)

	remoteConn, err := tls.DialWithDialer(&net.Dialer{
		Timeout: defaultTimeout,
	}, "tcp", remoteAddress, tlsConfig)
	if err != nil {
		logger.Warn("could not reach remote tls server", zap.Error(err))
		return
	}
	defer cmd.SilentClose(remoteConn)

	p := &proxy{
		log:  logger,
		wait: make(chan struct{}),
	}

	deadline := start.Add(connDeadline)
	_ = localConn.SetDeadline(deadline)
	_ = remoteConn.SetDeadline(deadline)

	go p.pipe(ctx, remoteConn, localConn, true)
	go p.pipe(ctx, localConn, remoteConn, false)

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
	wait          chan struct{}
	erred         uint32
}

func (p *proxy) pipe(ctx context.Context, dst io.Writer, src io.Reader, isLocal bool) {
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

	atomic.StoreUint32(&p.erred, 1)
	close(p.wait)
}
