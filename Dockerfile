FROM golang:latest

RUN go get github.com/davidk/chim

FROM alpine:edge
WORKDIR /usr/bin
COPY --from=0 /go/bin/chim .
CMD ["/usr/bin/chim"]
