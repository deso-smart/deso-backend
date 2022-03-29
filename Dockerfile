FROM alpine:latest AS backend

RUN apk update
RUN apk upgrade
RUN apk add --update go gcc g++ vips-dev

WORKDIR /deso/src/backend

COPY go.mod .
COPY go.sum .

RUN go mod download

# include backend src
COPY apis      apis
COPY config    config
COPY cmd       cmd
COPY miner     miner
COPY routes    routes
COPY countries countries
COPY main.go   .

# build backend
RUN GOOS=linux go build -mod=mod -a -installsuffix cgo -o bin/backend main.go

# create tiny image
FROM alpine:latest

RUN apk add --update vips-dev

COPY --from=backend /deso/src/backend/bin/backend /deso/bin/backend

ENTRYPOINT ["/deso/bin/backend"]
