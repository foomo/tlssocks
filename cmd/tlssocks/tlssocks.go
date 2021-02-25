package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"time"

	"github.com/armon/go-socks5"
	"github.com/foomo/htpasswd"
	"go.uber.org/zap"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
)

var logger *zap.Logger

func init() {
	l, _ := zap.NewProduction()
	logger = l
}

type socksAuthenticator struct {
	Destinations map[string]*Destination

	resolvedNames map[string][]string
}

func newSocksAuthenticator(destinations map[string]*Destination) (*socksAuthenticator, error) {
	suxa := &socksAuthenticator{
		Destinations: destinations,
	}
	names := make([]string, 0, len(destinations))
	for name := range destinations {
		names = append(names, name)
	}

	resolvedNames, err := resolveNames(names)
	if err != nil {
		return nil, err
	}
	suxa.resolvedNames = resolvedNames

	go func() {
		time.Sleep(time.Second * 10)

		resolvedNames, err := resolveNames(names)
		if err == nil {
			suxa.resolvedNames = resolvedNames
		} else {
			logger.Warn("could not resolve names: " + err.Error())
		}
	}()
	return suxa, nil
}

func resolveNames(names []string) (map[string][]string, error) {
	newResolvedNames := map[string][]string{}
	for _, name := range names {
		addrs, err := net.LookupHost(name)
		if err != nil {
			return nil, err
		}
		newResolvedNames[name] = addrs
	}
	return newResolvedNames, nil
}

func (suxx5 *socksAuthenticator) Allow(ctx context.Context, req *socks5.Request) (newCtx context.Context, allowed bool) {
	allowed = false
	newCtx = ctx
	zapTo := zap.String("to", req.DestAddr.String())
	zapUser := zap.String("for", req.AuthContext.Payload["Username"])

	for name, ips := range suxx5.resolvedNames {
		zapName := zap.String("name", name)
		for _, ip := range ips {
			if ip == req.DestAddr.IP.String() {
				destination, destinationOK := suxx5.Destinations[name]
				if destinationOK {
					for _, allowedPort := range destination.Ports {
						if allowedPort == req.DestAddr.Port {
							if len(destination.Users) == 0 {
								allowed = true
							}
							if !allowed {
								userNameInContext, userNameInContextOK := req.AuthContext.Payload["Username"]
								if !userNameInContextOK {
									// explicit user expected, but not found
									logger.Info("denied - no user found", zapName, zapTo)
									return
								}
								for _, userName := range destination.Users {
									if userName == userNameInContext {
										allowed = true
										break
									}
								}
								if !allowed {
									logger.Info("denied", zapName, zapTo, zapUser)
									return
								}
							}
							if allowed {
								logger.Info("allowed", zapName, zapTo, zapUser)

								allowed = true
								return
							}
						}
					}
				}
			}
		}
	}
	logger.Info("denied", zapTo, zapUser)
	return
}

type Credentials map[string]string

func (s Credentials) Valid(user, password string) bool {
	hashedPassword, ok := s[user]
	if !ok {
		return false
	}
	return nil == bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

func must(err error, comment ...interface{}) {
	if err != nil {
		logger.Fatal(fmt.Sprint(comment...), zap.Error(err))
	}
}

type Destination struct {
	Users []string
	Ports []int
}

func main() {
	defer logger.Sync()
	flagAddr := flag.String("addr", "", "where to listen like 127.0.0.1:8000")
	flagHtpasswdFile := flag.String("auth", "", "basic auth file")
	flagDestinationsFile := flag.String("destinations", "", "file with destinations config")
	flagCert := flag.String("cert", "", "path to server cert.pem")
	flagKey := flag.String("key", "", "path to server key.pem")
	flag.Parse()

	destinationBytes, err := ioutil.ReadFile(*flagDestinationsFile)
	must(err, "can not read destinations config")

	destinations := map[string]*Destination{}

	must(yaml.Unmarshal(destinationBytes, destinations), "can not parse destinations")

	passwordHashes, err := htpasswd.ParseHtpasswdFile(*flagHtpasswdFile)
	must(err, "basic auth file sucks")
	credentials := Credentials(passwordHashes)

	suxx5, err := newSocksAuthenticator(destinations)
	must(err)

	autenticator := socks5.UserPassAuthenticator{Credentials: credentials}

	conf := &socks5.Config{
		Rules:       suxx5,
		AuthMethods: []socks5.Authenticator{autenticator},
	}
	server, err := socks5.New(conf)
	must(err)

	logger.Info(
		"starting tls server",
		zap.String("addr", *flagAddr),
		zap.String("cert", *flagCert),
		zap.String("key", *flagKey),
	)

	cert, err := tls.LoadX509KeyPair(*flagCert, *flagKey)
	if err != nil {
		logger.Fatal("could not load server key pair", zap.Error(err))
	}

	listener, err := tls.Listen("tcp", *flagAddr, &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		logger.Fatal(
			"could not listen for tcp / tls",
			zap.String("addr", *flagAddr),
			zap.Error(err),
		)
	}
	logger.Fatal(
		"server failed",
		zap.Error(server.Serve(listener)),
	)
}
