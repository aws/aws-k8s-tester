.PHONY: docker

ACCOUNT_ID ?= $(shell aws sts get-caller-identity --query Account --output text)
REGION ?= us-west-2
ECR_HOST ?= amazonaws.com

# build custom "busybox" image
ORIGINAL_BUSYBOX_IMG ?= gcr.io/google-containers/busybox:latest
BUSYBOX_IMG_NAME ?= busybox
BUSYBOX_TAG ?= latest
busybox:
	docker pull $(ORIGINAL_BUSYBOX_IMG)
	docker tag $(ORIGINAL_BUSYBOX_IMG) $(ACCOUNT_ID).dkr.ecr.$(REGION).$(ECR_HOST)/$(BUSYBOX_IMG_NAME):$(BUSYBOX_TAG)
	eval $$(aws ecr get-login --registry-ids $(ACCOUNT_ID) --no-include-email --region $(REGION))
	docker push $(ACCOUNT_ID).dkr.ecr.$(REGION).$(ECR_HOST)/$(BUSYBOX_IMG_NAME):$(BUSYBOX_TAG);

# build custom "php-apache" image
PHP_APACHE_IMG_NAME ?= php-apache
PHP_APACHE_TAG ?= latest
php-apache:
	docker build --network host -t $(PHP_APACHE_IMG_NAME):$(PHP_APACHE_TAG) ./php-apache
	docker tag $(PHP_APACHE_IMG_NAME):$(PHP_APACHE_TAG) $(ACCOUNT_ID).dkr.ecr.$(REGION).$(ECR_HOST)/$(PHP_APACHE_IMG_NAME):$(PHP_APACHE_TAG)
	eval $$(aws ecr get-login --registry-ids $(ACCOUNT_ID) --no-include-email --region $(REGION))
	docker push $(ACCOUNT_ID).dkr.ecr.$(REGION).$(ECR_HOST)/$(PHP_APACHE_IMG_NAME):$(PHP_APACHE_TAG);

# build custom "stress" image
# TODO


