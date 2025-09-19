FROM golang:1.22 AS builder

WORKDIR /app

COPY go.mod ./
COPY /test-scripts ./
COPY . ./

RUN CGO_ENABLED=0 GOOS=linux go build -o go_js_test .

FROM node:20-slim

WORKDIR /app

COPY --from=builder /app/go_js_test .
COPY --from=builder /app/test-scripts ./test-scripts

ENTRYPOINT ["./go_js_test"]