package main

import (
	"crypto/tls"
	"flag"
	"io/ioutil"

	"github.com/armon/go-socks5"
	"github.com/foomo/htpasswd"
	"github.com/foomo/tlssocks/cmd"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

type Destination struct {
	Users []string
	Ports []int
}

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	flagAddr := flag.String("addr", "", "where to listen like 127.0.0.1:8000")
	flagHtpasswdFile := flag.String("auth", "", "basic auth file")
	flagDestinationsFile := flag.String("destinations", "", "file with destinations config")
	flagCert := flag.String("cert", "", "path to server cert.pem")
	flagKey := flag.String("key", "", "path to server key.pem")
	flagDisableBasicAuthCaching := flag.Bool("disable-basic-auth-caching", false, "if set disables caching of basic auth user and password")
	flag.Parse()

	destinationBytes, err := ioutil.ReadFile(*flagDestinationsFile)
	cmd.TryFatal(log, err, "can not read destinations config")

	destinations := map[string]*Destination{}

	cmd.TryFatal(log, yaml.Unmarshal(destinationBytes, destinations), "can not parse destinations")

	passwordHashes, err := htpasswd.ParseHtpasswdFile(*flagHtpasswdFile)
	cmd.TryFatal(log, err, "basic auth file sucks")
	credentials := Credentials{disableCaching: *flagDisableBasicAuthCaching, htpasswd: passwordHashes}

	suxx5, err := newAuthenticator(log, destinations)
	cmd.TryFatal(log, err, "newAuthenticator failed")

	autenticator := socks5.UserPassAuthenticator{Credentials: credentials}

	conf := &socks5.Config{
		Rules:       suxx5,
		AuthMethods: []socks5.Authenticator{autenticator},
	}
	server, err := socks5.New(conf)
	cmd.TryFatal(log, err, "socks5.New failed")

	log.Info(
		"starting tls server",
		zap.String("addr", *flagAddr),
		zap.String("cert", *flagCert),
		zap.String("key", *flagKey),
	)

	cert, err := tls.LoadX509KeyPair(*flagCert, *flagKey)
	cmd.TryFatal(log, err, "could not load server key pair")

	listener, err := tls.Listen("tcp", *flagAddr, &tls.Config{Certificates: []tls.Certificate{cert}})
	cmd.TryFatal(log, err, "could not listen for tcp / tls", zap.String("addr", *flagAddr))

	cmd.TryFatal(log, server.Serve(listener), "server failed")
}
