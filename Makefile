SHELL := /bin/bash

DOMAIN="*.example.com"
SAN=DNS:$(DOMAIN),DNS:localhost,DNS:127.0.0.1

SERVICE_VERSION ?= latest
BUILD_HASH?=`git rev-parse --short HEAD || "unknown"`

cert-create:
	rm -rf docker/local-test/tlssocks/certificate.*
	openssl req -newkey rsa:2048 -nodes -keyout docker/local-test/tlssocks/certificate.key -subj "/C=DE/ST=Bavaria/L=Munich/O=foomo/CN=$(DOMAIN)" -out docker/local-test/tlssocks/certificate.csr
	openssl x509 -req -extfile <(printf "subjectAltName=DNS:$(DOMAIN),DNS:localhost,DNS:127.0.0.1") -days 365 -signkey docker/local-test/tlssocks/certificate.key -in docker/local-test/tlssocks/certificate.csr -out docker/local-test/tlssocks/certificate.crt

cert-show:
	openssl x509 -in docker/local-test/tlssocks/certificate.crt -text -noout

cert-trust-on-mac:
	sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain docker/local-test/tlssocks/certificate.crt

docker.build.tlssocks:
	$(MAKE) SERVICE_NAME=tlssocks docker.build

docker.build.tcpproxy:
	$(MAKE) SERVICE_NAME=tcpproxy docker.build

docker.build.tlssocksproxy:
	$(MAKE) SERVICE_NAME=tlssocksproxy docker.build

docker.build:
	docker build . \
	  --build-arg SERVICE_NAME=${SERVICE_NAME} \
	  --build-arg SERVICE_VERSION=${SERVICE_VERSION} \
	  --build-arg BUILD_HASH=${BUILD_HASH} \
	  -t foomo/tcpproxy:${SERVICE_VERSION}

docker.build.all: docker.build.tlssocks docker.build.tcpproxy docker.build.tlssocksproxy

docker.push.all:
	docker push foomo/tlssocks:latest 
	docker push foomo/tcpproxy:latest
	docker push foomo/tlssocksproxy:latest

docker.test: cert-create cert-show
	cd docker/local-test && docker-compose up

