FROM golang:1.26 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG SERVICE
RUN CGO_ENABLED=0 go build -o /bin/service ./cmd/${SERVICE}

FROM gcr.io/distroless/static-debian12

COPY --from=builder /bin/service /service

ENTRYPOINT ["/service"]