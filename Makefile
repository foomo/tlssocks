SHELL := /bin/bash

DOMAIN="*.example.com"
SAN=DNS:$(DOMAIN),DNS:localhost,DNS:127.0.0.1

cert-create:
	rm -rf docker/local-test/tlssocks/certificate.*
	openssl req -newkey rsa:2048 -nodes -keyout docker/local-test/tlssocks/certificate.key -subj "/C=DE/ST=Bavaria/L=Munich/O=foomo/CN=$(DOMAIN)" -out docker/local-test/tlssocks/certificate.csr
	openssl x509 -req -extfile <(printf "subjectAltName=DNS:$(DOMAIN),DNS:localhost,DNS:127.0.0.1") -days 365 -signkey docker/local-test/tlssocks/certificate.key -in docker/local-test/tlssocks/certificate.csr -out docker/local-test/tlssocks/certificate.crt

cert-show:
	openssl x509 -in docker/local-test/tlssocks/certificate.crt -text -noout

cert-trust-on-mac:
	sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain docker/local-test/tlssocks/certificate.crt

docker-build-tlssocks:
	rm -vf cmd/tlssocks/tlssocks
	GOOS=linux go build -o cmd/tlssocks/tlssocks cmd/tlssocks/tlssocks.go
	cd cmd/tlssocks && docker build -t foomo/tlssocks:latest .
	rm -vf cmd/tlssocks/tlssocks

docker-build-tcpproxy:
	rm -vf cmd/tcpproxy/tcpproxy
	GOOS=linux go build -o cmd/tcpproxy/tcpproxy cmd/tcpproxy/tcpproxy.go
	cd cmd/tcpproxy && docker build -t foomo/tcpproxy:latest .
	rm -vf cmd/tcpproxy/tcpproxy

docker-build-tlssocksproxy:
	docker build -t foomo/tlssocksproxy:latest -f docker/tlssocksproxy/Dockerfile .

docker-build: docker-build-tlssocks docker-build-tcpproxy docker-build-tlssocksproxy
docker-push:
	docker push foomo/tlssocks:latest 
	docker push foomo/tcpproxy:latest
	docker push foomo/tlssocksproxy:latest

docker-local-test: cert-create cert-show 
	cd docker/local-test && docker-compose up

	