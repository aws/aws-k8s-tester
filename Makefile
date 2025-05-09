include ${BGO_MAKEFILE}

pre-release::
	go test -c -tags=e2e ./test/... -o $(GOBIN)
	# --flake-attempts flag was removed in: https://github.com/kubernetes-sigs/kubetest2/commit/7dee8a65e642615a9b753455f5b3c23dd3411a3d
	# so we'll pin to the previous commit for now: https://github.com/kubernetes-sigs/kubetest2/commit/d7fcb799ce84ceda66c8b9b1ec8eefcbe226f293
	# TODO: remove dependency on this flag
	go install sigs.k8s.io/kubetest2/...@v0.0.0-20231113220322-d7fcb799ce84
