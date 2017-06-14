MAINTAINER Pierre-Alexandre St-Jean <pa@stjean.me>

FROM alpine

RUN apk --update upgrade && \
    apk add curl ca-certificates && \
    update-ca-certificates && \
    rm -rf /var/cache/apk/*

COPY ./k8s-aws-elb-tagger /k8s-aws-elb-tagger

CMD ["/k8s-aws-elb-tagger"]
