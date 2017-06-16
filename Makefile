VERSION ?= $(TRAVIS_TAG)
REVISION ?= git-$(shell git rev-parse --short HEAD)
DOCKER_REPOSITORY ?= quay.io/coveo/k8s-aws-elb-tagger

k8s-aws-elb-tagger: k8s-aws-elb-tagger.go
	go build -o $@ $<

k8s-aws-elb-tagger.linux.amd64: k8s-aws-elb-tagger.go
	GOOS=linux GOARCH=amd64 go build -o $@ $<

k8s-aws-elb-tagger.darwin.amd64: k8s-aws-elb-tagger.go
	GOOS=darwin GOARCH=amd64 go build -o $@ $<

k8s-aws-elb-tagger.windows.amd64: k8s-aws-elb-tagger.go
	GOOS=windows GOARCH=amd64 go build -o $@ $<

k8s-aws-elb-tagger-all: k8s-aws-elb-tagger.windows.amd64 k8s-aws-elb-tagger.linux.amd64 k8s-aws-elb-tagger.darwin.amd64

.PHONY: test
test: k8s-aws-elb-tagger
	go test -cover -v ./
#	go list ./... | grep -v vendor | xargs go test

.PHONY: docker
docker: k8s-aws-elb-tagger.linux.amd64
	docker build -t $(DOCKER_REPOSITORY):$(REVISION) .

.PHONY: docker-push
docker-push: docker
	docker push $(DOCKER_REPOSITORY):$(REVISION)

docker-push-release: docker
	docker tag $(DOCKER_REPOSITORY):$(REVISION) $(DOCKER_REPOSITORY):$(VERSION)
	docker push $(DOCKER_REPOSITORY):$(VERSION)
