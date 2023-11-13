FROM hashicorp/terraform as terraform_builder

FROM golang:1.21.4-alpine as digger_builder

ARG DIGGER_BRANCH=develop
ARG OS=linux
ARG ARCH=amd64

RUN apk add --no-cache git
RUN git clone https://github.com/diggerhq/digger.git --depth 1 --branch ${DIGGER_BRANCH}
RUN cd digger && env GOOS=${OS} GOARCH=${ARCH} CGO_ENABLED=0 go build -o digger ./cmd/digger && chmod +x digger && mv digger /usr/local/bin/digger

FROM alpine:latest

COPY --from=terraform_builder /bin/terraform /usr/local/bin/terraform
COPY --from=digger_builder /usr/local/bin/digger /usr/local/bin/digger


