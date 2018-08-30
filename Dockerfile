FROM golang as builder
WORKDIR /go/src/github.com/dat1041988/ssp-backend
COPY . .
RUN go get -v ./server

FROM centos:7
COPY --from=builder /go/bin/server /usr/local/bin
EXPOSE 8080
ENTRYPOINT server
