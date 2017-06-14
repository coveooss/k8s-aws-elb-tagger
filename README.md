# K8S - AWS ELB Service Tagger

![Coveo](https://img.shields.io/badge/Coveo-awesome-f58020.svg)
[![Build Status](https://travis-ci.org/coveo/k8s-aws-elb-tagger.svg?branch=master)](https://travis-ci.org/coveo/k8s-aws-elb-tagger)

Manage AWS tags on Kubernetes created loadbalancer type services

## Usage

### Deploy

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: k8s-aws-elb-tagger
  labels:
    app: k8s-aws-elb-tagger
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: k8s-aws-elb-tagger
    spec:
      containers:
        - image: quay.io/coveo/k8s-aws-elb-tagger:v1.0.0
          name: k8s-aws-elb-tagger
```

### Add annotations to your service

Add a tag that starts with `aws-tag/` with its value to the value you want: `aws-tag/<tag-key>=<tag-value>`

example: `aws-tag/owner=John Doe`

If you wan't to use keys that do not match the k8s allowed annotation regex `[A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]` eg: `coveo:owner` there is another way to define tags using 2 annotations:

- `aws-tag-key/<id>=<tag-key>`
- `aws-tag-value/<id>=<tag-value>`

example: 

```
aws-tag-key/1=coveo:owner
aws-tag-value/1=johndoe@example.com
```

#### In your service spec

To your load balancer service spec:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  annotations:
    "aws-tag/owner": John Doe
    "aws-tag-key/1": coveo:owner
    "aws-tag-value/1": johndoe@example.com
spec:
  selector:
    app: my-app
  ports:
  - name: https
    port: 443
    protocol: TCP
    targetPort: https
  type: LoadBalancer
```

### Required AWS Right

## Develop

```sh
# Get dependency tool
go get -u github.com/golang/dep/cmd/dep
# Get the dependencies
dep ensure
```

## Nice endpoints

- /healthz
- /stats
- /metrics
- /debug/pprofs
- /debug/vars