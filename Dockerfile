# Use the offical Golang image to create a build artifact.
# This is based on Debian and sets the GOPATH to /go.
# https://hub.docker.com/_/golang
FROM golang:1.16.2-alpine as builder
RUN apk add --no-cache gcc libc-dev git

ARG version=develop

# Copy local code to the container image.
WORKDIR /go/src/github.com/keptn-contrib/argo-service

# Force the go compiler to use modules
ENV GO111MODULE=on
ENV GOPROXY=https://proxy.golang.org
ENV BUILDFLAGS=""

# Copy `go.mod` for definitions and `go.sum` to invalidate the next layer
# in case of a change in the dependencies
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy local code to the container image.
COPY . .

# `skaffold debug` sets SKAFFOLD_GO_GCFLAGS to disable compiler optimizations
ARG SKAFFOLD_GO_GCFLAGS

# Build the command inside the container.
# (You may fetch or manage dependencies here, either manually or with a tool like "godep".)
RUN GOOS=linux go build -ldflags '-linkmode=external' -gcflags="${SKAFFOLD_GO_GCFLAGS}" -v -o argo-service ./cmd/

# Use a Docker multi-stage build to create a lean production image.
# https://docs.docker.com/develop/develop-images/multistage-build/#use-multi-stage-builds
FROM alpine:3.15
# Install extra packages
# See https://github.com/gliderlabs/docker-alpine/issues/136#issuecomment-272703023

RUN    apk update && apk upgrade \
	&& apk add ca-certificates libc6-compat \
	&& update-ca-certificates \
	&& rm -rf /var/cache/apk/*

ARG version
ENV version $version

ENV env=production
ARG debugBuild

# Set the Kubernetes version as found in the UCP Dashboard or API
ARG KUBEECTL_VERSION=v1.16.2

# Get the kubectl binary.
RUN wget https://storage.googleapis.com/kubernetes-release/release/$KUBEECTL_VERSION/bin/linux/amd64/kubectl && \
    chmod +x ./kubectl && \
    mv ./kubectl /bin/kubectl

RUN wget https://github.com/argoproj/argo-rollouts/releases/download/v0.10.2/kubectl-argo-rollouts-linux-amd64 && \
    chmod +x ./kubectl-argo-rollouts-linux-amd64 && \
    mv ./kubectl-argo-rollouts-linux-amd64 /bin/kubectl-argo-rollouts


# Copy the binary to the production image from the builder stage.
COPY --from=builder /go/src/github.com/keptn-contrib/argo-service/argo-service /argo-service

# required for external tools to detect this as a go binary
ENV GOTRACEBACK=all

# Run the web service on container startup.
CMD ["/argo-service"]
