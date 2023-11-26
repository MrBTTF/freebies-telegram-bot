FROM alpine:latest as builder
RUN apk update && apk upgrade && apk add --no-cache ca-certificates
RUN update-ca-certificates

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY build/app /usr/bin/app

ENTRYPOINT [ "/usr/bin/app" ]