FROM alpine:3.22
LABEL authors="will"

ARG BIN_NAME
ARG BIN_PATH

RUN apk add --no-cache ca-certificates

COPY $BIN_PATH /usr/local/bin/$BIN_NAME

ENTRYPOINT ["/usr/local/bin/$BIN_NAME"]
