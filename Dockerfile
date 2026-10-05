ARG ENV=development

FROM golang:1.27.1-alpine AS builder
WORKDIR /src    
COPY src/go.mod src/go.sum ./
COPY ./certs ./certs
RUN go mod download

COPY src .

RUN CGO_ENABLED=0 GOOS=linux go build -o main .

FROM alpine:latest  
RUN apk --no-cache add ca-certificates
RUN apk add --no-cache curl iproute2 net-tools bind-tools iputils && mkdir -p /app/
WORKDIR /app

COPY --from=builder /src/main .
COPY --from=builder /src/certs ./certs
COPY --from=builder /src/certs /usr/local/share/ca-certificates
RUN update-ca-certificates

COPY --from=builder /src/templates ./templates
COPY --from=builder /src/static ./static

EXPOSE 8081

CMD ["./main", "-env", "${ENV}"]
