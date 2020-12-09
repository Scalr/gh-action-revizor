FROM golang:1.15-alpine AS builder
WORKDIR /go/src/github.com/scalr/gh-action-revizor
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -o revizor .

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /go/src/github.com/scalr/gh-action-revizor/revizor /bin/revizor
ENTRYPOINT ["revizor"]
