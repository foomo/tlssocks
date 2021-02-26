package main

import (
	"context"
	"crypto/tls"
	"flag"
	"io/ioutil"
	"net"
	"time"

	"github.com/armon/go-socks5"
	"github.com/foomo/htpasswd"
	"github.com/foomo/tlssocks/cmd"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v2"
)

type authenticator struct {
	log           *zap.Logger
	Destinations  map[string]*Destination
	resolvedNames map[string][]string
}

func newAuthenticator(log *zap.Logger, destinations map[string]*Destination) (*authenticator, error) {
	sa := &authenticator{
		log:          log,
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
	sa.resolvedNames = resolvedNames

	go func() {
		time.Sleep(time.Second * 10)

		resolvedNames, err := resolveNames(names)
		if err == nil {
			sa.resolvedNames = resolvedNames
		} else {
			log.Warn("could not resolve names", zap.Error(err))
		}
	}()
	return sa, nil
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

func (sa *authenticator) Allow(ctx context.Context, req *socks5.Request) (newCtx context.Context, allowed bool) {
	allowed = false
	newCtx = ctx
	zapTo := zap.String("to", req.DestAddr.String())
	zapUser := zap.String("for", req.AuthContext.Payload["Username"])

	for name, ips := range sa.resolvedNames {
		zapName := zap.String("name", name)
		for _, ip := range ips {
			if ip == req.DestAddr.IP.String() {
				destination, destinationOK := sa.Destinations[name]
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
									sa.log.Info("denied - no user found", zapName, zapTo)
									return
								}
								for _, userName := range destination.Users {
									if userName == userNameInContext {
										allowed = true
										break
									}
								}
								if !allowed {
									sa.log.Info("denied", zapName, zapTo, zapUser)
									return
								}
							}
							if allowed {
								sa.log.Info("allowed", zapName, zapTo, zapUser)

								allowed = true
								return
							}
						}
					}
				}
			}
		}
	}
	sa.log.Info("denied", zapTo, zapUser)
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
	flag.Parse()

	destinationBytes, err := ioutil.ReadFile(*flagDestinationsFile)
	cmd.TryFatal(log, err, "can not read destinations config")

	destinations := map[string]*Destination{}

	cmd.TryFatal(log, yaml.Unmarshal(destinationBytes, destinations), "can not parse destinations")

	passwordHashes, err := htpasswd.ParseHtpasswdFile(*flagHtpasswdFile)
	cmd.TryFatal(log, err, "basic auth file sucks")
	credentials := Credentials(passwordHashes)

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
