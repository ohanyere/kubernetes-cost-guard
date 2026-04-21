FROM golang:1.22-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/kubernetes-cost-guard \
    ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /

COPY --from=builder /out/kubernetes-cost-guard /kubernetes-cost-guard

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/kubernetes-cost-guard"]
