FROM golang:1.22-alpine as builder
WORKDIR /app
COPY main.go .
# Create a static binary
RUN CGO_ENABLED=0 go build -o server main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/server .
CMD ["./server"]