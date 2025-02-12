FROM golang:1.24-bullseye AS compile

WORKDIR /build

COPY go.mod .
COPY go.sum .

RUN go mod download
COPY ./ ./

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /build/ws cmd/ws/main.go

FROM alpine:latest

WORKDIR /app

RUN apk add tini

ENTRYPOINT ["tini", "--"]

COPY --from=compile /build/ws /usr/bin

EXPOSE 80

ARG APP_ENV
ENV APP_ENV=${APP_ENV:-production}

CMD ["ws", "--env", "${APP_ENV}"]
