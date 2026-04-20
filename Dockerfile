ARG BASE_IMAGE=alpine:3.22
FROM ${BASE_IMAGE}
LABEL authors="will"

ARG BIN_PATH

RUN apk add --no-cache ca-certificates

COPY $BIN_PATH /usr/local/bin/mailmover

ENTRYPOINT ["/usr/local/bin/mailmover"]
