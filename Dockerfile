FROM golang:1.16-alpine AS builder
RUN apk add git
WORKDIR /app
COPY . .
RUN go get
RUN go build

FROM alpine:3.13
WORKDIR /app
COPY --from=builder /app/cluster-operator /app/
CMD ["./cluster-operator"]