FROM golang:1.24.3-alpine AS builder
WORKDIR /work
COPY . .
RUN go test -c -o /bin/example.test -tags e2e

FROM ubuntu:22.04
RUN apt-get update && \
    apt-get install -y ca-certificates
WORKDIR /work
COPY --from=builder /bin/example.test /bin/
# COPY --from=builder /work/fixtures .
ENTRYPOINT ["/bin/example.test"]
CMD ["-test.v"]
