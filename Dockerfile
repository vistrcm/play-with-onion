FROM --platform=$BUILDPLATFORM golang:1.19 as builder
ARG TARGETOS
ARG TARGETARCH
ARG GITHUB_SHA
ENV APP_VERSION=$GITHUB_SHA
ENV GOFLAGS="-mod=vendor"

WORKDIR /go/src/github.com/vistrcm/play-with-onion/
COPY ./ .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -v -o play-with-onion -ldflags "-w -s -X main.revision=${APP_VERSION:-devimage}" .

# build image phase
FROM scratch

#COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /go/src/github.com/vistrcm/play-with-onion/play-with-onion /play-with-onion
# array in etrypoint is a dirty hack to be able to pass parameters via CMD later
ENTRYPOINT ["/play-with-onion"]