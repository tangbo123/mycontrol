FROM golang:1.18 as builder

WORKDIR /app

COPY ./ ./

RUN go env -w GO111MODULE=on

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app main.go

FROM debian:buster-20220822

WORKDIR /app

COPY --from=builder /app/app ./

ENTRYPOINT ["./app"]