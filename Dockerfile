FROM golang:1.17-alpine3.15 AS builder

RUN apk --no-cache add gcc g++ vips-dev upx

WORKDIR /usr/src/deso/backend

COPY go.mod .
COPY go.sum .

RUN go mod download && go mod verify

COPY apis apis
COPY cmd cmd
COPY config config
COPY countries countries
COPY miner miner
COPY routes routes
COPY main.go .

RUN GOOS=linux go build -ldflags "-s -w" -o /usr/local/bin/deso-backend main.go
RUN upx /usr/local/bin/deso-backend

FROM alpine:3.15

RUN apk --no-cache add vips

COPY --from=builder /usr/local/bin/deso-backend /usr/local/bin/deso-backend

ENTRYPOINT ["/usr/local/bin/deso-backend"]

CMD ["run"]
