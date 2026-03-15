# ---- Build stage ----
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod ./
RUN go mod download && go mod tidy

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server .

# ---- Final stage ----
FROM scratch

COPY --from=builder /app/server /server

EXPOSE 8080
ENTRYPOINT ["/server"]
