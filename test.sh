#!/bin/sh
set -eu

docker build -f Dockerfile.test -t go_js_test .
docker run --rm -e GO_JS_DIFFERENTIAL="${GO_JS_DIFFERENTIAL:-0}" go_js_test:latest
