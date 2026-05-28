FROM golang:1.26-alpine@sha256:91eda9776261207ea25fd06b5b7fed8d397dd2c0a283e77f2ab6e91bfa71079d AS builder
WORKDIR /src
RUN apk add --no-cache upx
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -trimpath -o bin/config-sidecar cmd/root.go
RUN upx --best --lzma bin/config-sidecar

FROM gcr.io/distroless/static@sha256:3592aa8171c77482f62bbc4164e6a2d141c6122554ace66e5cc910cadb961ff0
COPY --from=builder /src/bin/config-sidecar /config-sidecar
ENTRYPOINT ["/config-sidecar"]
