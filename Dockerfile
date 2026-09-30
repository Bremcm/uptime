FROM golang:1.26 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG SERVICE
RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,target=/root/.cache/go-build,sharing=locked \
    CGO_ENABLED=0 go build -o /bin/service ./cmd/${SERVICE}

FROM gcr.io/distroless/static-debian12

COPY --from=builder /bin/service /service

ENTRYPOINT ["/service"]