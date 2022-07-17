# docker build -t mailgo_hw2 .
FROM golang:1.18.3-alpine
COPY . .
ENV GO111MODULE=auto
ENV CGO_ENABLED=0
RUN go test -v