
.PHONY: docker

ACCOUNT_ID ?= $(shell aws sts get-caller-identity --query Account --output text)
REGION ?= us-west-2
ECR_HOST ?= amazonaws.com

# build custom "busybox" image
ORIGINAL_BUSYBOX_IMG ?= gcr.io/google-containers/busybox:latest
ECR_BUSYBOX_IMG_NAME ?= busybox
ECR_BUSYBOX_TAG ?= latest
busybox:
	docker pull $(ORIGINAL_BUSYBOX_IMG)
	docker tag $(ORIGINAL_BUSYBOX_IMG) $(ACCOUNT_ID).dkr.ecr.$(REGION).$(ECR_HOST)/$(ECR_BUSYBOX_IMG_NAME):$(ECR_BUSYBOX_TAG)
	eval $$(aws ecr get-login --registry-ids $(ACCOUNT_ID) --no-include-email --region $(REGION))
	docker push $(ACCOUNT_ID).dkr.ecr.$(REGION).$(ECR_HOST)/$(ECR_BUSYBOX_IMG_NAME):$(ECR_BUSYBOX_TAG);

# build custom "php-apache" image
ECR_PHP_APACHE_IMG_NAME ?= php-apache
ECR_PHP_APACHE_TAG ?= latest
php-apache:
	docker build --network host -t $(ECR_PHP_APACHE_IMG_NAME):$(ECR_PHP_APACHE_TAG) ./k8s-tester/php-apache
	docker tag $(ECR_PHP_APACHE_IMG_NAME):$(ECR_PHP_APACHE_TAG) $(ACCOUNT_ID).dkr.ecr.$(REGION).$(ECR_HOST)/$(ECR_PHP_APACHE_IMG_NAME):$(ECR_PHP_APACHE_TAG)
	eval $$(aws ecr get-login --registry-ids $(ACCOUNT_ID) --no-include-email --region $(REGION))
	docker push $(ACCOUNT_ID).dkr.ecr.$(REGION).$(ECR_HOST)/$(ECR_PHP_APACHE_IMG_NAME):$(ECR_PHP_APACHE_TAG);

# build custom "stress" image
ECR_K8S_TESTER_STRESS_IMG_NAME ?= k8s-tester-stress
ECR_K8S_TESTER_STRESS_TAG ?= latest
k8s-tester-stress:
	DOCKER_BUILDKIT=0 docker build --network host -t $(ECR_K8S_TESTER_STRESS_IMG_NAME):$(ECR_K8S_TESTER_STRESS_TAG) -f ./Dockerfile.k8s-tester-stress .
	docker tag $(ECR_K8S_TESTER_STRESS_IMG_NAME):$(ECR_K8S_TESTER_STRESS_TAG) $(ACCOUNT_ID).dkr.ecr.$(REGION).$(ECR_HOST)/$(ECR_K8S_TESTER_STRESS_IMG_NAME):$(ECR_K8S_TESTER_STRESS_TAG)
	eval $$(aws ecr get-login --registry-ids $(ACCOUNT_ID) --no-include-email --region $(REGION))
	docker push $(ACCOUNT_ID).dkr.ecr.$(REGION).$(ECR_HOST)/$(ECR_K8S_TESTER_STRESS_IMG_NAME):$(ECR_K8S_TESTER_STRESS_TAG);
















#
#
#
#
#
#
#
#
#
#
#
# old targets
AKT_IMG_NAME ?= aws/aws-k8s-tester
AKT_TAG ?= latest
AKT_AWS_ACCOUNT_ID ?= $(shell aws sts get-caller-identity --query Account --output text)
AKT_AWS_REGION ?= us-west-2
AKT_DISTRIBUTION = linux-amd64
AKT_S3_BUCKET = s3://eks-prow
AKT_S3_PREFIX ?= $(AKT_S3_BUCKET)/bin/aws-k8s-tester
AKT_S3_PATH ?= $(AKT_S3_PREFIX)/aws-k8s-tester-$(AKT_TAG)-$(AKT_DISTRIBUTION)
AKT_ECR_HOST ?= amazonaws.com

WHAT ?= aws-k8s-tester
TARGETS ?= $(shell uname | awk '{print tolower($0)}')

build:
	WHAT=$(WHAT) TARGETS=$(TARGETS) RELEASE_VERSION=$(AKT_TAG) ./hack/build.sh

clean:
	rm -rf ./bin ./_tmp
	find **/*.generated.yaml -print0 | xargs -0 rm -f || true
	find **/*.coverprofile -print0 | xargs -0 rm -f || true

# Publish all components
publish: s3-release docker-release

docker-publish: docker-build docker-push

docker-build:
	@if [ ! -f "./_tmp/clusterloader2" ]; then echo "downloading clusterloader2"; aws s3 cp --region us-west-2 s3://aws-k8s-tester-public/clusterloader2-linux-amd64 ./_tmp/clusterloader2; else echo "skipping downloading clusterloader2"; fi;
	@if [ ! -d "${HOME}/go/src/k8s.io/perf-tests" ]; then echo "cloning perf-tests"; mkdir -p ${HOME}/go/src/k8s.io; pushd ${HOME}/go/src/k8s.io; git clone https://github.com/kubernetes/perf-tests.git; popd; else echo "skipping cloning perf-tests"; fi
	@if [ ! -d "./_tmp/clusterloader2-testing-load" ]; then echo "copying clusterloader2/testing/load"; cp -rf ${HOME}/go/src/k8s.io/perf-tests/clusterloader2/testing/load ./_tmp/clusterloader2-testing-load; else echo "skipping copying clusterloader2/testing/load"; fi
	docker build --network host -t $(AKT_IMG_NAME):$(AKT_TAG) --build-arg RELEASE_VERSION=$(AKT_TAG) .
	docker tag $(AKT_IMG_NAME):$(AKT_TAG) $(AKT_AWS_ACCOUNT_ID).dkr.ecr.$(AKT_AWS_REGION).$(AKT_ECR_HOST)/$(AKT_IMG_NAME):$(AKT_TAG)
	docker run --rm -it $(AKT_IMG_NAME):$(AKT_TAG) aws --version

# release latest aws-k8s-tester to ECR
docker-push:
	eval $$(aws ecr get-login --registry-ids $(AKT_AWS_ACCOUNT_ID) --no-include-email --region $(AKT_AWS_REGION))
	docker push $(AKT_AWS_ACCOUNT_ID).dkr.ecr.$(AKT_AWS_REGION).$(AKT_ECR_HOST)/$(AKT_IMG_NAME):$(AKT_TAG);

# release latest aws-k8s-tester to S3
s3-publish: build s3-push

s3-push:
ifeq ("$(AKT_TAG)","latest")
	echo "skipping uploading tagged $(AKT_TAG) aws-k8s-tester binary";
else
	echo "uploading tagged $(AKT_TAG) aws-k8s-tester binary"; aws s3 cp --region $(AKT_AWS_REGION) ./bin/aws-k8s-tester-$(AKT_TAG)-linux-amd64 $(AKT_S3_PATH);
endif
	aws s3 cp --region $(AKT_AWS_REGION) ./bin/aws-k8s-tester-$(AKT_TAG)-linux-amd64 $(AKT_S3_PREFIX)/aws-k8s-tester-latest-linux-amd64
	aws s3 ls s3://eks-prow/bin/aws-k8s-tester/

ORIGINAL_BUSYBOX_IMG ?= gcr.io/google-containers/busybox:latest
BUSYBOX_IMG_NAME ?= busybox
BUSYBOX_TAG ?= latest

docker-busybox:
	docker pull $(ORIGINAL_BUSYBOX_IMG)
	docker tag $(ORIGINAL_BUSYBOX_IMG) $(AKT_AWS_ACCOUNT_ID).dkr.ecr.$(AKT_AWS_REGION).$(AKT_ECR_HOST)/$(BUSYBOX_IMG_NAME):$(BUSYBOX_TAG)

docker-push-busybox:
	eval $$(aws ecr get-login --registry-ids $(AKT_AWS_ACCOUNT_ID) --no-include-email --region $(AKT_AWS_REGION))
	docker push $(AKT_AWS_ACCOUNT_ID).dkr.ecr.$(AKT_AWS_REGION).$(AKT_ECR_HOST)/$(BUSYBOX_IMG_NAME):$(BUSYBOX_TAG);


PHP_APACHE_IMG_NAME ?= php-apache
PHP_APACHE_TAG ?= latest

docker-php-apache:
	docker build --network host -t $(PHP_APACHE_IMG_NAME):$(PHP_APACHE_TAG) ./images/php-apache
	docker tag $(PHP_APACHE_IMG_NAME):$(PHP_APACHE_TAG) $(AKT_AWS_ACCOUNT_ID).dkr.ecr.$(AKT_AWS_REGION).$(AKT_ECR_HOST)/$(PHP_APACHE_IMG_NAME):$(PHP_APACHE_TAG)

docker-push-php-apache:
	eval $$(aws ecr get-login --registry-ids $(AKT_AWS_ACCOUNT_ID) --no-include-email --region $(AKT_AWS_REGION))
	docker push $(AKT_AWS_ACCOUNT_ID).dkr.ecr.$(AKT_AWS_REGION).$(AKT_ECR_HOST)/$(PHP_APACHE_IMG_NAME):$(PHP_APACHE_TAG);

CL2_IMAGE_NAME ?= clusterloader2
CL2_TAG ?= latest
CL2_FULL_IMAGE_PATH ?= $(AKT_AWS_ACCOUNT_ID).dkr.ecr.$(AKT_AWS_REGION).$(AKT_ECR_HOST)/$(CL2_IMAGE_NAME):$(AKT_TAG)

docker-build-clusterloader2:
	docker build -f ./images/clusterloader2/Dockerfile ./images/clusterloader2 -t $(CL2_FULL_IMAGE_PATH)

docker-push-clusterloader2:
	docker push $(CL2_FULL_IMAGE_PATH)

docker-release-clusterloader2: docker-build-clusterloader2 docker-push-clusterloader2
