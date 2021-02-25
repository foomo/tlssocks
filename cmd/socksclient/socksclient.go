package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"go.uber.org/zap"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()
	flagSocksServer := flag.String("socks-server", "socks5://test:test@127.0.0.1:8000", "addr of socks server like socks://user:pass@127.0.0.1:8000")
	flag.Parse()

	if len(flag.Args()) != 1 {
		log.Fatal("usage: " + os.Args[0] + " -socks-server=127.0.0.1:8000 http://www.google.com/")
	}

	urlToFetch := flag.Arg(0)

	proxyURL, errProxyURL := url.Parse(*flagSocksServer)
	if errProxyURL != nil {
		log.Fatal("invalid proxy server:", zap.Error(errProxyURL))
	}

	response, err := newClient(proxyURL).Get(urlToFetch)
	if err != nil {
		log.Fatal("could not GET", zap.Error(err), zap.String("url", urlToFetch))
	}
	defer response.Body.Close()

	respBytes, err := httputil.DumpResponse(response, true)
	if err != nil {
		log.Fatal("failed httputil.DumpResponse", zap.Error(err))
	}

	fmt.Println(string(respBytes))
}

func newClient(proxyURL *url.URL) *http.Client {
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
	return &http.Client{
		Transport: transport,
	}
}
