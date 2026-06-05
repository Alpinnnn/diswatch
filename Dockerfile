# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

WORKDIR /src
RUN apk add --no-cache ca-certificates

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=arm64
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/diswatch ./cmd/diswatch
RUN mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=builder --chown=nonroot:nonroot /out/diswatch /diswatch
COPY --from=builder --chown=nonroot:nonroot /out/data /app/data

ENV DISWATCH_ADDR=:8080
ENV DISWATCH_DATA_DIR=/app/data

EXPOSE 8080
VOLUME ["/app/data"]
ENTRYPOINT ["/diswatch"]
