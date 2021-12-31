#!/usr/bin/env bash
set -eu
org=localhost
domain=localhost:3001

sudo trust anchor --remove configs/certs/ca.crt || true

openssl genpkey -algorithm RSA -out configs/certs/ca.key

openssl req -x509 -key configs/certs/ca.key \
                  -out configs/certs/ca.crt \
                  -subj "/CN=$org/O=$org"

openssl genpkey -algorithm RSA -out "configs/certs/key.pem"

openssl req -new -key "configs/certs/key.pem" \
                 -out "configs/certs/$domain.csr" \
                 -subj "/CN=$domain/O=$org"

openssl x509 -req -in "configs/certs/$domain.csr" \
    -out "configs/certs/cert.pem" \
    -days 365  \
    -CA configs/certs/ca.crt -CAkey configs/certs/ca.key -CAcreateserial \
    -extfile <(cat <<END
basicConstraints = CA:FALSE
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer
subjectAltName=DNS:localhost
END
    )

sudo trust anchor configs/certs/ca.crt
rm "configs/certs/$domain.csr" configs/certs/ca.srl

