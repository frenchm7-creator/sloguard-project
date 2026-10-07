# syntax=docker/dockerfile:1
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
ARG VERSION=dev
ENV CGO_ENABLED=0
# Static, stripped binaries; -trimpath keeps build paths out of the artifact.
RUN go vet ./... \
 && go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/ledger    ./cmd/ledger \
 && go build -trimpath -ldflags="-s -w" -o /out/loadgen   ./cmd/loadgen \
 && go build -trimpath -ldflags="-s -w" -o /out/alertsink ./cmd/alertsink

# distroless/static: no shell, no package manager, CA certs included, runs as non-root.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ /
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/ledger"]
