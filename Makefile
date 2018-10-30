
.PHONY: build
build:
	go build -v ./cmd/aws-k8s-tester

clean:
	rm -f ./aws-k8s-tester
	find **/*.generated.yaml -print0 | xargs -0 rm -f || true
	find **/*.coverprofile -print0 | xargs -0 rm -f || true
	find **/*.log -print0 | xargs -0 rm -f || true
