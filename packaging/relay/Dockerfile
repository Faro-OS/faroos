FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/relay ./cmd/relay
COPY internal/relayserver ./internal/relayserver
COPY internal/relaytransport ./internal/relaytransport
COPY internal/stunserver ./internal/stunserver
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/faroos-relay ./cmd/relay

FROM alpine:3.22
RUN addgroup -S -g 10001 faroos-relay \
    && adduser -S -D -H -u 10001 -G faroos-relay faroos-relay \
    && install -d -o faroos-relay -g faroos-relay -m 0750 /data
COPY --from=build /out/faroos-relay /usr/local/bin/faroos-relay
USER faroos-relay
ENV FAROOS_RELAY_ADDR=:8080 \
	FAROOS_RELAY_DB=/data/relay.db \
	FAROOS_STUN_ADDR=:3478
EXPOSE 8080
EXPOSE 3478/udp
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=5s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/faroos-relay"]
