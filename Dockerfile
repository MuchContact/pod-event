FROM golang:1.15 AS build-env
ADD . /src/events.com/pod
ENV GOPATH /:/src/events.com/pod/vendor
ENV GO111MODULE on
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /src/events.com/pod
RUN apt-get update -y && apt-get install gcc ca-certificates
RUN make


FROM alpine:3.11.6

COPY --from=build-env /src/events.com/pod/pod-event /
COPY --from=build-env /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENV TZ "Asia/Shanghai"
RUN apk add --no-cache tzdata
#COPY deploy/entrypoint.sh /
RUN addgroup -g 1000 nonroot && \
    adduser -u 1000 -D -H -G nonroot nonroot && \
    chown -R nonroot:nonroot /pod-event
USER nonroot:nonroot

ENTRYPOINT ["/pod-event"]

