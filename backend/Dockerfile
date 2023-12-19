FROM golang:1.21 as builder
ARG COMMIT_SHA
RUN echo "commit sha: ${COMMIT_SHA}"

# Set the working directory
WORKDIR $GOPATH/src/github.com/diggerhq/cloud

# Copy all required source, blacklist files that are not required through `.dockerignore`
COPY . .

# Get the vendor library
RUN go version

# RUN vgo install

# https://github.com/ethereum/go-ethereum/issues/2738
# Build static binary "-getmode=vendor" does not work with go-ethereum
RUN go build -ldflags="-X 'main.Version=${COMMIT_SHA}'"

# Multi-stage build will just copy the binary to an alpine image.
FROM ubuntu:22.04 as runner
ARG COMMIT_SHA
WORKDIR /app

RUN apt-get update && apt-get install -y ca-certificates && apt-get install -y git && apt-get clean all
RUN update-ca-certificates

RUN echo "commit sha: ${COMMIT_SHA}"

# Set gin to production
#ENV GIN_MODE=release

# Expose the running port
EXPOSE 3000

# Copy the binary to the corresponding folder
COPY --from=builder /go/src/github.com/diggerhq/cloud/cloud .
ADD templates ./templates

# Run the binary
CMD ["/app/cloud"]
