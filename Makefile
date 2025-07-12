include ${BGO_MAKEFILE}

pre-release::
	go test -c -tags=e2e ./test/... -o $(GOBIN)
	go install sigs.k8s.io/kubetest2/...@latest

update-deps:
	for SCRIPT in ./hack/update-*.sh; do \
    	"$$SCRIPT" ; \
	done

