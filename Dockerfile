FROM golang:1.7.3-alpine

COPY . /go/src/github.com/gronnbeck/httperf

RUN go install -v github.com/gronnbeck/httperf/...
