VERSION ?= 1.0.0
REVISION ?= git-$(shell git rev-parse --short HEAD)

k8s-aws-elb-tagger: cmd/k8s-aws-elb-tagger/main.go
	go build -o k8s-aws-elb-tagger cmd/k8s-aws-elb-tagger/main.go

k8s-aws-elb-tagger.linux.amd64: cmd/k8s-aws-elb-tagger/main.go
	GOOS=linux GOARCH=amd64 go build -o k8s-aws-elb-tagger.linux.amd64 cmd/k8s-aws-elb-tagger/main.go

test:
	go list ./... | grep -v vendor | xargs go test

.PHONY: docker
docker: k8s-aws-elb-tagger.linux.amd64
	docker build -t quay.io/coveo/k8s-aws-elb-tagger:$(REVISION) .

.PHONY: docker-push
docker-push: docker
	docker tag quay.io/coveo/k8s-aws-elb-tagger:$(REVISION) quay.io/coveo/k8s-aws-elb-tagger:$(VERSION)
	docker push quay.io/coveo/k8s-aws-elb-tagger:$(REVISION)
	docker push quay.io/coveo/k8s-aws-elb-tagger:$(VERSION)
