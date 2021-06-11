FROM alpine:3.12.4

RUN apk --no-cache add ca-certificates
COPY dist/mascaras_linux_amd64/mascaras /usr/local/bin/mascaras
WORKDIR /
ENTRYPOINT ["/usr/local/bin/mascaras"]
