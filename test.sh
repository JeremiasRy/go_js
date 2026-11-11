#!/bin/sh
set -eu

docker build -f Dockerfile.test -t go_js_test .
docker run --rm go_js_test:latest
