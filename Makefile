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

docker-local-test: cert-create cert-show 
	cd docker/local-test && docker-compose up -d

docker-build:
	foomo