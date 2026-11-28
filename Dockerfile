# Google's Docker Hub mirror: release builds on shared GitHub runners hit
# Docker Hub's anonymous pull limit. The CA bundle is the same on every
# architecture, so this stage runs on the build host and needs no emulation.
FROM --platform=$BUILDPLATFORM mirror.gcr.io/library/alpine:3.21 AS certs
RUN apk add --no-cache ca-certificates

FROM scratch
ENV HOME=/root
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY switchboard /switchboard

EXPOSE 3847
ENTRYPOINT ["/switchboard"]
CMD ["--port", "3847"]
