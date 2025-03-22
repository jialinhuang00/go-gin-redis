# Step 1: golang server builder, for step 2 COPY
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod tidy

COPY . .

RUN go build -o main .

# Step 2: main build image here
FROM alpine:latest

WORKDIR /root/

# copy binary
COPY --from=builder /app/main .

# install redis
RUN apk add --no-cache redis

# just a reminder, not forcing to decide which port should open
EXPOSE 80

# run redis daemon and run go
CMD redis-server --daemonize yes && ./main