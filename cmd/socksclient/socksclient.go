package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"go.uber.org/zap"
)

var logger *zap.Logger

func init() {
	l, _ := zap.NewProduction()
	logger = l
}

func main() {
	defer logger.Sync()
	flagSocksServer := flag.String("socks-server", "socks5://test:test@127.0.0.1:8000", "addr of socks server like socks://user:pass@127.0.0.1:8000")
	flag.Parse()

	if len(flag.Args()) != 1 {
		logger.Fatal("usage: " + os.Args[0] + " -socks-server=127.0.0.1:8000 http://www.google.com/")
	}

	urlToFetch := flag.Arg(0)

	proxyURL, errProxyURL := url.Parse(*flagSocksServer)
	if errProxyURL != nil {
		logger.Fatal("invalid proxy server:", zap.Error(errProxyURL))
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	client := &http.Client{
		Transport: transport,
	}
	response, errGet := client.Get(urlToFetch)
	if errGet != nil {
		logger.Fatal("could not GET", zap.String("url", urlToFetch))
	}
	defer response.Body.Close()
	bodyBytes, errRead := ioutil.ReadAll(response.Body)
	if errRead != nil {
		logger.Fatal("could not read response body", zap.Error(errRead))
	}
	fmt.Println(string(bodyBytes))
}
