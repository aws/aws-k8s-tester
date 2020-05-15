
.PHONY: release
release:
	./scripts/build.sh

clean:
	rm -rf ./bin
	find **/*.generated.yaml -print0 | xargs -0 rm -f || true
	find **/*.coverprofile -print0 | xargs -0 rm -f || true

IMG_NAME ?= aws/aws-k8s-tester
TAG ?= latest

ACCOUNT_ID ?= $(aws sts get-caller-identity --query Account --output text)
REGION ?= us-west-2

docker: clean
	docker build --network host -t $(IMG_NAME):$(TAG) --build-arg RELEASE_VERSION=$(TAG) .
	docker tag $(IMG_NAME):$(TAG) $(ACCOUNT_ID).dkr.ecr.$(REGION).amazonaws.com/$(IMG_NAME):$(TAG)
	docker run --rm -it $(IMG_NAME):$(TAG) aws --version

# e.g.
# make docker-push ACCOUNT_ID=${YOUR_ACCOUNT_ID} TAG=latest
docker-push: docker
	eval $$(aws ecr get-login --registry-ids $(ACCOUNT_ID) --no-include-email --region $(REGION))
	docker push $(ACCOUNT_ID).dkr.ecr.$(REGION).amazonaws.com/$(IMG_NAME):$(TAG);

