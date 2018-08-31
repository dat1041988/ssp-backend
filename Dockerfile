FROM golang as builder
# Get specific version of gin-jwt
RUN go get gopkg.in/appleboy/gin-jwt.v2 && cd /go/src/gopkg.in/appleboy/gin-jwt.v2 && git checkout 25dbc558978a0b3caf0358d6b70b7a8eb87bdffb
WORKDIR /go/src/github.com/dat1041988/ssp-backend
COPY . .
RUN go get -v ./server

FROM centos:7
COPY --from=builder /go/bin/server /usr/local/bin
EXPOSE 8080
ENTRYPOINT server
