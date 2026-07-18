FROM golang:1.26.5-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./main.go

FROM alpine:latest  
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/main .

# Copy static assets or HTML templates if your website uses them
# COPY --from=builder /app/templates ./templates
# COPY --from=builder /app/static ./static

EXPOSE 9000

CMD ["./main"]
