FROM golang:1.22 as builder-b
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o goapp

FROM alpine:latest
WORKDIR /app
COPY --from=builder-b /app/goapp .
EXPOSE 8082
CMD ["./goapp"]