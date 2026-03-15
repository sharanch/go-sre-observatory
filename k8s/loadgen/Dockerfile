FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY main.go .
RUN go mod init loadgen && go build -o loadgen .

FROM scratch
COPY --from=builder /app/loadgen /loadgen
ENTRYPOINT ["/loadgen"]
