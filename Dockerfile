FROM golang:1.26-alpine AS builder
WORKDIR /src
RUN apk add --no-cache upx
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -trimpath -o bin/config-sidecar cmd/root.go
RUN upx --best --lzma bin/config-sidecar

FROM gcr.io/distroless/static
COPY --from=builder /src/bin/config-sidecar /config-sidecar
ENTRYPOINT ["/config-sidecar"]
