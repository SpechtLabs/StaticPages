FROM alpine:latest

LABEL org.opencontainers.image.title="StaticPages"
LABEL org.opencontainers.image.source="https://github.com/SpechtLabs/StaticPages"
LABEL org.opencontainers.image.description="StaticPages is a simple server implementation to host your static pages with support for preview URLs."
LABEL org.opencontainers.image.licenses="Apache-2.0"
LABEL org.opencontainers.image.authors="SpechtLabs <cedi@specht-labs.de>"
LABEL org.opencontainers.image.url="https://staticpages.specht-labs.de"
LABEL org.opencontainers.image.documentation="https://staticpages.specht-labs.de/docs"
LABEL org.opencontainers.image.vendor="SpechtLabs"

ARG TARGETPLATFORM
COPY ${TARGETPLATFORM}/staticpages /bin/staticpages

ENTRYPOINT ["/bin/staticpages"]
CMD [ "serve" ]

EXPOSE     8099
EXPOSE     50051
