FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/bin/rmtv .

FROM alpine:3.16

WORKDIR /app

COPY --from=builder /app/bin/rmtv /usr/local/bin/rmtv

RUN mkdir data

VOLUME /app/data

ENTRYPOINT ["rmtv"]