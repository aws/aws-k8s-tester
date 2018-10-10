
.PHONY: build
build:
	go build -v ./cmd/awstester

clean:
	rm -f ./awstester
	find **/*.generated.yaml -print0 | xargs -0 rm -f || true
	find **/*.coverprofile -print0 | xargs -0 rm -f || true
	find **/*.log -print0 | xargs -0 rm -f || true
