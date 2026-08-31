FROM golang:1.26.5-alpine AS builder
WORKDIR /app
COPY src/go.mod src/go.sum ./
COPY src/certs ./certs
RUN go mod download

COPY src .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -env development -o main ./main.go

FROM alpine:latest  
RUN apk --no-cache add ca-certificates
RUN apk add --no-cache curl iproute2 net-tools bind-tools iputils && mkdir -p /root/
WORKDIR /root/

COPY --from=builder /app/main .

# Copy static assets or HTML templates if your website uses them
# COPY --from=builder /app/templates ./templates
# COPY --from=builder /app/static ./static

EXPOSE 8081

CMD ["./main"]
