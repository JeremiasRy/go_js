FROM golang:1.22 AS test_builder

WORKDIR /app

COPY ./test/go.mod ./
COPY ./test ./
COPY ./test/test-scripts ./
COPY ./test/benchmark-scripts ./

RUN CGO_ENABLED=0 GOOS=linux go build -o go_js_test .

FROM golang:1.22 AS runtime_builder

WORKDIR /app

COPY ./runtime/go.mod ./
COPY ./runtime ./

RUN CGO_ENABLED=0 GOOS=linux go build -o go_js .

FROM node:20-slim

WORKDIR /app

COPY --from=test_builder /app/go_js_test .
COPY --from=test_builder /app/test-scripts ./test-scripts
COPY --from=test_builder /app/benchmark-scripts ./benchmark-scripts
COPY --from=runtime_builder /app/go_js .

ENTRYPOINT ["./go_js_test"]