FROM golang:1.22 as builder
ARG COMMIT_SHA
RUN echo "commit sha: ${COMMIT_SHA}"

# Set the working directory
WORKDIR $GOPATH/src/github.com/diggerhq/drift

# Copy all required source, blacklist files that are not required through `.dockerignore`
COPY . .

# Get the vendor library
RUN go version

# RUN vgo install

# https://github.com/ethereum/go-ethereum/issues/2738
# Build static binary "-getmode=vendor" does not work with go-ethereum

RUN go build -ldflags="-X 'main.Version=${COMMIT_SHA}'" -o driftapp_exe ./

# Multi-stage build will just copy the binary to an alpine image.
FROM ubuntu:24.04 as runner
ENV ATLAS_VERSION v0.16.0
ARG COMMIT_SHA
WORKDIR /app

RUN apt-get update && apt-get install -y ca-certificates curl && apt-get install -y git && apt-get clean all
RUN update-ca-certificates

RUN echo "commit sha: ${COMMIT_SHA}"


# Set gin to production
#ENV GIN_MODE=release

# Expose the running port
EXPOSE 3000

# Copy the binary to the corresponding folder
COPY --from=builder /go/src/github.com/diggerhq/drift/driftapp_exe /app/driftapp
COPY --from=builder /go/src/github.com/diggerhq/drift/scripts/entrypoint.sh /app/entrypoint.sh

# Run the binary
ENTRYPOINT ["/bin/bash", "/app/entrypoint.sh"]
