include ${BGO_MAKEFILE}

pre-release::
	go test -c -tags=e2e ./test/... -o $(GOBIN)
	go install sigs.k8s.io/kubetest2/...@latest
