# Build the binary
FROM golang:1.14.4 as builder
WORKDIR /go/src/k8s.io
RUN apt-get update && apt-get -y install rsync
RUN git clone https://github.com/kubernetes/kubernetes
WORKDIR /go/src/k8s.io/kubernetes
RUN make WHAT=cmd/kubemark

# Copy into a thin image
FROM gcr.io/distroless/static:latest
WORKDIR /
COPY --from=builder /go/src/k8s.io/kubernetes/_output/bin/kubemark .
ENTRYPOINT ["/kubemark"]