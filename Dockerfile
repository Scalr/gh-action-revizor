FROM golang:1.15 AS builder
WORKDIR /go/src/github.com/scalr/gh-action-revizor
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -o revizor .

FROM golang:1.15
COPY --from=builder /go/src/github.com/scalr/gh-action-revizor/revizor /bin/revizor
ENTRYPOINT ["revizor"]
