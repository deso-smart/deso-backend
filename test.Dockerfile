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

ENTRYPOINT ["go", "test", "-v", "github.com/deso-smart/deso-backend/v2/routes"]
