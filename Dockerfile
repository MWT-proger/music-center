
#####################################################
### Build UI bundles
FROM node:16-alpine AS jsbuilder
WORKDIR /src
COPY ui/package.json ui/package-lock.json ./
RUN npm ci
COPY ui/ .
RUN npm run build


#####################################################
### Build executable
FROM golang:1.20.5-alpine AS gobuilder

# Download build tools
RUN mkdir -p /src/ui/build
RUN apk --no-cache add taglib-dev gcc g++ libc-dev librdkafka-dev pkgconf

# Download project dependencies
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Copy UI bundle, build executable
COPY --from=jsbuilder /src/build/* /src/ui/build/
COPY --from=jsbuilder /src/build/static/css/* /src/ui/build/static/css/
COPY --from=jsbuilder /src/build/static/js/* /src/ui/build/static/js/

RUN rm -rf /src/build/css /src/build/js
RUN rm -rf ~/.cache/go-build
RUN go build  -tags=netgo -o myapp .

#####################################################
### Build Final Image
FROM alpine:3.18.2 as release

LABEL maintainer="dry.wats@gmail.com"

WORKDIR /src

COPY --from=gobuilder /src/myapp /

RUN apk --no-cache add taglib-dev gcc g++ libc-dev librdkafka-dev pkgconf ffmpeg
RUN ffmpeg -buildconf

VOLUME ["/data", "/music"]
ENV ND_MUSICFOLDER /music
ENV ND_DATAFOLDER /data
ENV ND_SCANINTERVAL 1m
ENV ND_TRANSCODINGCACHESIZE 100MB
ENV ND_SESSIONTIMEOUT 30m
ENV ND_LOGLEVEL info
ENV ND_PORT 4533
ENV ND_DOMENNAME navidrome.ru

EXPOSE ${ND_PORT}
HEALTHCHECK CMD wget -O- http://localhost:${ND_PORT}/ping || exit 1

RUN mkdir -p /src/data/cache
RUN chmod 777 /src/data/cache
ENTRYPOINT ["/myapp"]