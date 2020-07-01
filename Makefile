
AKT_IMG_NAME ?= aws/aws-k8s-tester
AKT_TAG ?= latest
AKT_AWS_ACCOUNT_ID ?= $(shell aws sts get-caller-identity --query Account --output text)
AKT_AWS_REGION ?= us-west-2
AKT_DISTRIBUTION = linux-amd64
AKT_S3_BUCKET = s3://eks-prow
AKT_S3_PREFIX ?= $(AKT_S3_BUCKET)/bin/aws-k8s-tester
AKT_S3_PATH ?= $(AKT_S3_PREFIX)/aws-k8s-tester-$(AKT_TAG)-$(AKT_DISTRIBUTION)

clean:
	rm -rf ./bin ./_tmp
	find **/*.generated.yaml -print0 | xargs -0 rm -f || true
	find **/*.coverprofile -print0 | xargs -0 rm -f || true

docker:
	aws s3 cp --region us-west-2 s3://aws-k8s-tester-public/clusterloader2-linux-amd64 ./_tmp/clusterloader2
	cp -rf ${HOME}/go/src/k8s.io/perf-tests/clusterloader2/testing/load ./_tmp/clusterloader2-testing-load
	docker build --network host -t $(AKT_IMG_NAME):$(AKT_TAG) --build-arg RELEASE_VERSION=$(AKT_TAG) .
	docker tag $(AKT_IMG_NAME):$(AKT_TAG) $(AKT_AWS_ACCOUNT_ID).dkr.ecr.$(AKT_AWS_REGION).amazonaws.com/$(AKT_IMG_NAME):$(AKT_TAG)
	docker run --rm -it $(AKT_IMG_NAME):$(AKT_TAG) aws --version

build:
	RELEASE_VERSION=$(AKT_TAG) ./scripts/build.sh

# Release latest Aws-K8s-Tester to ECR.
docker-push: docker
	eval $$(aws ecr get-login --registry-ids $(AKT_AWS_ACCOUNT_ID) --no-include-email --region $(AKT_AWS_REGION))
	docker push $(AKT_AWS_ACCOUNT_ID).dkr.ecr.$(AKT_AWS_REGION).amazonaws.com/$(AKT_IMG_NAME):$(AKT_TAG);

# Release latest Aws-K8s-Tester to S3.
s3-push: build
	aws s3 cp --region $(AKT_AWS_REGION) ./bin/aws-k8s-tester-$(AKT_TAG)-linux-amd64 $(AKT_S3_PATH)
	aws s3 ls s3://eks-prow/bin/aws-k8s-tester/

release: clean s3-push docker-push
