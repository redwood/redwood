#!/bin/sh

openssl req \
    -x509 \
    -nodes \
    -newkey rsa:2048 \
    -keyout server1.key \
    -out server1.crt \
    -days 3650 \
    -subj "/C=GB/ST=London/L=London/O=Global Security/OU=IT Department/CN=*"


openssl req \
    -x509 \
    -nodes \
    -newkey rsa:2048 \
    -keyout server2.key \
    -out server2.crt \
    -days 3650 \
    -subj "/C=GB/ST=London/L=London/O=Global Security/OU=IT Department/CN=*"
