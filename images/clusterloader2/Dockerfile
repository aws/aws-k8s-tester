FROM golang:1.14.4 AS builder
WORKDIR /go/src/k8s.io
RUN git clone https://github.com/kubernetes/perf-tests
WORKDIR /go/src/k8s.io/perf-tests/clusterloader2
RUN GOPROXY=direct go build -o ./clusterloader ./cmd

FROM gcr.io/distroless/base:latest
WORKDIR /
COPY --from=builder /go/src/k8s.io/perf-tests/clusterloader2/clusterloader .
ENTRYPOINT ["/clusterloader"]
