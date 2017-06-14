k8s-aws-elb-tagger.linux.amd64: main.go
	GOOS=linux GOARCH=amd64 go build -o k8s-aws-elb-tagger.linux.amd64 .

docker: k8s-aws-elb-tagger.linux.amd64
	docker build -t quay.io/coveo/k8s-aws-elb-tagger:v1.9.9 .
	docker push quay.io/coveo/k8s-aws-elb-tagger:v1.0.0
