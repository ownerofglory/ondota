# Build phase
FROM golang:1.26-alpine AS build

RUN apk add --no-cache make git
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

ARG BUILD_VERSION=dev
ENV CGO_ENABLED=0
RUN make build VERSION=$BUILD_VERSION

# Run phase
FROM alpine:3.22

RUN apk add --no-cache postgresql17-client ca-certificates \
    && addgroup -S -g 10001 ondota \
    && adduser -S -D -u 10001 -G ondota ondota

COPY --from=build /app/bin/ondota-server /usr/local/bin/ondota-server
COPY migrations /migrations
COPY scripts/migrate.sh /usr/local/bin/migrate
RUN chmod 0555 /usr/local/bin/migrate
USER ondota
EXPOSE 8080 8081

CMD ["ondota-server"]
