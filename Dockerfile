FROM golang:1.14-alpine AS builder
RUN apk add git
WORKDIR /app
COPY . .
RUN go get
RUN go build

FROM alpine:3.12
WORKDIR /app
COPY --from=builder /app/cluster-operator /app/
CMD ["./cluster-operator"]