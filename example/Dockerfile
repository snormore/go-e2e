FROM ubuntu:22.04

RUN apt-get update && \
    apt-get install -y ca-certificates

WORKDIR /work

ARG TEST_BIN
ARG TEST_ASSETS

COPY $TEST_BIN /bin/run-test
COPY $TEST_ASSETS /work/

ENTRYPOINT ["/bin/run-test"]

CMD ["-test.v"]
