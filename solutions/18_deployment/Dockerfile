# syntax=docker/dockerfile:1
#
# Build from the repository root (the module lives there):
#   docker build -f exercises/18_deployment/Dockerfile \
#     --build-arg VERSION=1.2.3 --build-arg COMMIT=$(git rev-parse --short HEAD) -t goref-service .

# Build stage: pinned toolchain matching go.mod.
FROM golang:1.27.4 AS build
WORKDIR /src

# Dependencies first: this layer is reused until go.mod or go.sum changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
      -ldflags "-s -w -X goref/exercises/18_deployment.Version=${VERSION} -X goref/exercises/18_deployment.Commit=${COMMIT}" \
      -o /out/goref-service ./exercises/18_deployment/cmd/goref-service

# Runtime stage: static binary, no shell, no package manager, non-root (uid 65532).
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/goref-service /usr/local/bin/goref-service
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/goref-service"]
CMD ["serve"]
