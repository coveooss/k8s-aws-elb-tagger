VERSION ?= 1.0.0
REVISION ?= git-$(shell git rev-parse --short HEAD)

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
	go test ./
#	go list ./... | grep -v vendor | xargs go test

.PHONY: docker
docker: k8s-aws-elb-tagger.linux.amd64
	docker build -t quay.io/coveo/k8s-aws-elb-tagger:$(REVISION) .

.PHONY: docker-push
docker-push: docker
	docker tag quay.io/coveo/k8s-aws-elb-tagger:$(REVISION) quay.io/coveo/k8s-aws-elb-tagger:$(VERSION)
	docker push quay.io/coveo/k8s-aws-elb-tagger:$(REVISION)
	docker push quay.io/coveo/k8s-aws-elb-tagger:$(VERSION)
