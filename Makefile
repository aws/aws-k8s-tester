include ${BGO_MAKEFILE}

pre-release::
	go test -c -tags=e2e ./test/... -o $(GOBIN)
	# --flake-attempts flag was removed in: https://github.com/kubernetes-sigs/kubetest2/commit/7dee8a65e642615a9b753455f5b3c23dd3411a3d
	# so we'll pin to the previous commit for now: https://github.com/kubernetes-sigs/kubetest2/commit/d7fcb799ce84ceda66c8b9b1ec8eefcbe226f293
	# TODO: remove dependency on this flag
	go install sigs.k8s.io/kubetest2/...@v0.0.0-20231113220322-d7fcb799ce84

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
GOPROXY ?= $(shell go env GOPROXY)

.PHONY: bin
bin: kubetest2-eksapi kubetest2-eksapi-janitor kubetest2-eksctl kubetest2-tester-ginkgo-v1 kubetest2-tester-multi

.PHONY: kubetest2-eksapi
kubetest2-eksapi:
	GO111MODULE=on CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) GOPROXY=$(GOPROXY) go build \
                -trimpath \
                -o=./bin/kubetest2-eksapi \
                cmd/kubetest2-eksapi/main.go

.PHONY: kubetest2-eksapi-janitor
kubetest2-eksapi-janitor:
	GO111MODULE=on CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) GOPROXY=$(GOPROXY) go build \
                -trimpath \
                -o=./bin/kubetest2-eksapi-janitor \
                cmd/kubetest2-eksapi-janitor/main.go

.PHONY: kubetest2-eksctl
kubetest2-eksctl:
	GO111MODULE=on CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) GOPROXY=$(GOPROXY) go build \
                -trimpath \
                -o=./bin/kubetest2-eksctl \
                cmd/kubetest2-eksctl/main.go

.PHONY: kubetest2-tester-ginkgo-v1
kubetest2-tester-ginkgo-v1:
	GO111MODULE=on CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) GOPROXY=$(GOPROXY) go build \
                -trimpath \
                -o=./bin/kubetest2-tester-ginkgo-v1\
                cmd/kubetest2-tester-ginkgo-v1/main.go

.PHONY: kubetest2-tester-multi
kubetest2-tester-multi:
	GO111MODULE=on CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) GOPROXY=$(GOPROXY) go build \
                -trimpath \
                -o=./bin/kubetest2-tester-multi\
                cmd/kubetest2-tester-multi/main.go
