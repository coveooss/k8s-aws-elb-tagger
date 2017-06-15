MAINTAINER Pierre-Alexandre St-Jean <pastjean@coveo.com>

FROM alpine

RUN apk --update upgrade && \
    apk add curl ca-certificates && \
    update-ca-certificates && \
    rm -rf /var/cache/apk/*

COPY ./k8s-aws-elb-tagger.linux.amd64 /k8s-aws-elb-tagger

CMD ["/k8s-aws-elb-tagger"]
