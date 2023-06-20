
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
FROM golang:1.20.5 AS gobuilder

# Download build tools
RUN mkdir -p /src/ui/build
# RUN apt-get install build-base git
RUN apt update && apt install --force-yes -y go-bindata libtag1-dev glibc-source
# RUN apt update && apt install --force-yes -y 
# RUN apt update && apt install --force-yes -y 
# RUN apt install go-bindata pkgconfig taglib-dev gcompat

# Download project dependencies
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

# Copy source, test it
COPY . .
# RUN go test ./...

# Copy UI bundle, build executable
COPY --from=jsbuilder /src/build/* /src/ui/build/
COPY --from=jsbuilder /src/build/static/css/* /src/ui/build/static/css/
COPY --from=jsbuilder /src/build/static/js/* /src/ui/build/static/js/
RUN rm -rf /src/build/css /src/build/js
RUN go-bindata -prefix ui/build -tags embed -nocompress -pkg assets -o assets/embedded_gen.go ui/build/...
RUN rm -rf ~/.cache/go-build
RUN go build -tags=netgo

RUN apt update && apt install --force-yes -y ffmpeg
RUN ffmpeg -buildconf

VOLUME ["/data", "/music"]
ENV ND_MUSICFOLDER /music
ENV ND_DATAFOLDER /data
ENV ND_SCANINTERVAL 1m
ENV ND_TRANSCODINGCACHESIZE 100MB
ENV ND_SESSIONTIMEOUT 30m
ENV ND_LOGLEVEL info
ENV ND_PORT 4533

EXPOSE ${ND_PORT}
HEALTHCHECK CMD wget -O- http://localhost:${ND_PORT}/ping || exit 1

RUN mkdir -p /src/data/cache
RUN chmod 777 /src/data/cache

# WORKDIR /app

# #####################################################
# ### Build Final Image
# FROM debian:unstable-slim as release
# LABEL maintainer="deluan@navidrome.org"

# COPY --from=gobuilder /src/navidrome /app/

# # Install ffmpeg and output build config
# RUN apt update && apt install --force-yes -y ffmpeg
# RUN ffmpeg -buildconf

# VOLUME ["/data", "/music"]
# ENV ND_MUSICFOLDER /music
# ENV ND_DATAFOLDER /data
# ENV ND_SCANINTERVAL 1m
# ENV ND_TRANSCODINGCACHESIZE 100MB
# ENV ND_SESSIONTIMEOUT 30m
# ENV ND_LOGLEVEL info
# ENV ND_PORT 4533

# EXPOSE ${ND_PORT}
# HEALTHCHECK CMD wget -O- http://localhost:${ND_PORT}/ping || exit 1
# WORKDIR /app
