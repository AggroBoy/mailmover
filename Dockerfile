FROM alpine:3.22
LABEL authors="will"

RUN apk add --no-cache ca-certificates

COPY mailmover /usr/local/bin/mailmover

ENTRYPOINT ["/usr/local/bin/mailmover"]
