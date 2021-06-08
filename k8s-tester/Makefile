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
	docker build --network host -t $(ECR_PHP_APACHE_IMG_NAME):$(ECR_PHP_APACHE_TAG) ./php-apache
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
