include ${BGO_MAKEFILE}

pre-release::
	go test -c -tags=e2e ./test/... -o $(GOBIN)