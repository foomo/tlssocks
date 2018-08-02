package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"io"
	"net"
	"time"

	"go.uber.org/zap"
)

var logger *zap.Logger

func init() {
	l, _ := zap.NewProduction()
	logger = l
}

func copyShit(name string, chanErr chan error, dst io.Writer, src io.Reader) {
	_, errCopy := io.Copy(dst, src)
	if errCopy != nil {
		chanErr <- errors.New(name + " " + errCopy.Error())
	}
	chanErr <- nil
}

func main() {
	defer logger.Sync()
	flagInsecureSkipVerify := flag.Bool("insecure-skip-verify", false, "allow insecure skipping of peer verification, when talking to the server")
	flagAddr := flag.String("addr", "", "address to listen to like 0.0.0.0:8001")
	flagAddrServer := flag.String("server", "", "address of the tls socks server like 0.0.0.0:8000")
	flag.Parse()
	logger.Info(
		"starting socks proxy to listen on addr and forward requests to server",
		zap.String("addr", *flagAddr),
		zap.String("server", *flagAddrServer),
	)

	socks5Listener, errListenSocks5 := net.Listen("tcp", *flagAddr)
	if errListenSocks5 != nil {
		logger.Fatal(
			"error listening for incoming socks connections",
			zap.Error(errListenSocks5),
		)

	}
	defer socks5Listener.Close()

	var tlsConfig *tls.Config
	if *flagInsecureSkipVerify {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: true,
		}

	}
	logger.Warn("running without verification of the tls server - this is dangerous")
	for {
		socksConn, errAccept := socks5Listener.Accept()
		if errAccept != nil {
			logger.Fatal(
				"error accepting incoming connections",
				zap.Error(errAccept),
			)
		}
		logger.Info(
			"socks client connected",
			zap.String("from", socksConn.RemoteAddr().String()),
		)
		go (func(socksConn net.Conn) {
			start := time.Now()
			conn, errDial := tls.Dial("tcp", *flagAddrServer, tlsConfig)
			if errDial != nil {
				logger.Warn(
					"could not reach tls server",
					zap.Error(errDial),
				)
				return
			}
			defer conn.Close()

			chanErr := make(chan error)
			go copyShit("conn->socksConn", chanErr, conn, socksConn)
			go copyShit("socksConn->conn", chanErr, socksConn, conn)
			errCopy := <-chanErr

			if errCopy != nil {
				switch true {
				case errCopy == io.ErrUnexpectedEOF,
					errCopy == io.ErrClosedPipe,
					errCopy == io.EOF,
					errCopy.Error() == "broken pipe":
					logger.Info(
						"an error occured, while copying data",
						zap.Error(errCopy),
					)
				}
			}
			logger.Info(
				"served request",
				zap.Duration("dur", time.Now().Sub(start)),
				zap.Error(errCopy),
			)
		})(socksConn)
	}

}
