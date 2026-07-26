# A local preview of the web page, with stand-ins for Cognito, the API and S3.
# Build from the repository root, since the image needs both preview/ and
# frontend/:
#
#   docker build -f preview/Dockerfile -t doi-dropbox-preview .
#   docker run --rm -p 8080:8080 doi-dropbox-preview
#
# Or just `make preview`.

FROM golang:1.26-alpine AS build
WORKDIR /src

# The preview server has no dependencies outside the standard library, so
# there is no module download step to cache.
COPY preview/go.mod ./preview/
COPY preview/main.go ./preview/

RUN cd preview && CGO_ENABLED=0 go build -o /out/preview .

# A scratch image would work too, but alpine makes it possible to shell in and
# look around when the preview itself is what is misbehaving.
FROM alpine:3.21

# Run as a non-root user. Nothing here is exposed to the internet, but a
# container that only serves files has no reason to be root either.
RUN adduser -D -u 10001 preview
USER preview

WORKDIR /app
COPY --from=build /out/preview /app/preview
COPY frontend /app/frontend

ENV FRONTEND_DIR=/app/frontend
ENV PORT=8080
EXPOSE 8080

CMD ["/app/preview"]
