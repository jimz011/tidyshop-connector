# The build stage runs on the builder's own architecture and cross-compiles, so a
# multi-arch image never runs the Go toolchain under emulation.
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod *.go ./
RUN go test ./...
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /tidyshop-connector .

FROM alpine:3.22
LABEL org.opencontainers.image.title="TidyShop Connector" \
      org.opencontainers.image.description="Self-hosted family sharing server for the TidyShop Android app" \
      org.opencontainers.image.source="https://github.com/jimz011/tidyshop-connector" \
      org.opencontainers.image.licenses="MPL-2.0"
COPY --from=build /tidyshop-connector /tidyshop-connector
VOLUME ["/data"]
EXPOSE 8787
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD ["/tidyshop-connector", "healthcheck"]
ENTRYPOINT ["/tidyshop-connector"]
