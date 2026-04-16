FROM alpine:3.21 AS certs
RUN apk add --no-cache ca-certificates

FROM scratch
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY switchboard /switchboard

EXPOSE 3847
ENTRYPOINT ["/switchboard"]
CMD ["--port", "3847"]
