package main

import (
	"context"
	"crypto/tls"
	"flag"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/foomo/tlssocks"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

var (
	logger *zap.Logger
)

const (
	defaultTimeout           = 180 * time.Second
	defaultPrometheusAddress = ":9200"
)

func init() {
	l, _ := zap.NewProduction()
	logger = l
}

func copyData(streamName string, dst io.Writer, src io.Reader) error {
	_, err := io.Copy(dst, src)
	if err != nil {
		return errors.Wrapf(err, "failed to copy stream %s", streamName)
	}
	return nil
}

func main() {
	defer logger.Sync()

	flagInsecureSkipVerify := flag.Bool("insecure-skip-verify", false, "allow insecure skipping of peer verification, when talking to the server")
	flagAddr := flag.String("addr", "0.0.0.0:8080", "address to listen to like 0.0.0.0:8001")
	flagAddrServer := flag.String("server", "", "address of the tls socks server like 0.0.0.0:8000")
	flag.Parse()

	logger.Info(
		"Starting socks proxy to listen on addr and forward requests to server",
		zap.String("addr", *flagAddr),
		zap.String("server", *flagAddrServer),
	)

	socks5Listener, errListenSocks5 := net.Listen("tcp", *flagAddr)
	if errListenSocks5 != nil {
		logger.Fatal(
			"Error listening for incoming socks connections",
			zap.Error(errListenSocks5),
		)
	}

	defer silentClose(socks5Listener)

	var tlsConfig *tls.Config

	if *flagInsecureSkipVerify {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
		logger.Warn("Running without verification of the tls server - this is dangerous")
	}
	ctx := context.Background()

	go runPrometheusHandler(ctx, defaultPrometheusAddress)

	for {
		socksConn, err := socks5Listener.Accept()
		if err != nil {
			logger.Fatal(
				"error accepting incoming connections",
				zap.Error(err),
			)
		}
		logger.Info(
			"socks client connected",
			zap.String("from", socksConn.RemoteAddr().String()),
		)
		go serve(ctx, socksConn, *flagAddrServer, tlsConfig)
	}
}

func serve(ctx context.Context, srcConn io.ReadWriteCloser, destinationAddress string, tlsConfig *tls.Config) {
	// Recover if a panic occurs
	defer recoverAndLogPanic()
	defer silentClose(srcConn)

	// Cancel context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	start := time.Now()

	dstConn, errDial := tls.DialWithDialer(&net.Dialer{
		KeepAlive: -1,
		Timeout:   defaultTimeout,
	}, "tcp", destinationAddress, tlsConfig)

	if errDial != nil {
		logger.Warn(
			"could not reach tls server",
			zap.Error(errDial),
		)
		return
	}
	defer silentClose(dstConn)

	group, gctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		srcConn := tlssocks.NewBufferedReader(gctx, srcConn)
		return copyData("conn->socksConn", dstConn, srcConn)
	})
	group.Go(func() error {
		dstConn := tlssocks.NewBufferedReader(gctx, dstConn)
		return copyData("socksConn->conn", srcConn, dstConn)
	})

	if err := group.Wait(); err != nil {
		switch {
		case err == io.ErrUnexpectedEOF,
			err == io.ErrClosedPipe,
			err == io.EOF,
			err.Error() == "broken pipe":
			logger.Warn("Error occurred, while copying data", zap.Error(err))
		default:
			logger.Error("Unexpected error occurred while copying the data", zap.Error(err))
		}
	}

	logger.Info(
		"request served",
		zap.Duration("duration", time.Now().Sub(start)),
	)
}

func recoverAndLogPanic() {
	if r := recover(); r != nil {
		var err error
		switch x := r.(type) {
		case string:
			err = errors.New(x)
		case error:
			err = x
		default:
			// Fallback err (per specs, error strings should be lowercase w/o punctuation
			err = errors.New("unknown panic")
		}
		logger.Error("Panic occurred in serve thread", zap.Error(err))
	}
}

func runPrometheusHandler(_ context.Context, address string) {
	h := http.NewServeMux()
	h.Handle("/metrics", promhttp.Handler())
	logger.Fatal("Failed to start prometheus handler", zap.Error(http.ListenAndServe(address, h)))
}

func silentClose(closer io.Closer) {
	_ = closer.Close()
}
