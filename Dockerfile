FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/sni-unlock ./cmd/sniproxy-dns

FROM scratch
COPY --from=build /out/sni-unlock /sni-unlock
ENTRYPOINT ["/sni-unlock"]
CMD ["-config", "/config.yaml"]
