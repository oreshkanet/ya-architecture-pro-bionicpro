FROM golang:1.24.3-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o bionicpro-auth .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/bionicpro-auth .
EXPOSE 8001
CMD ["./bionicpro-auth"]
