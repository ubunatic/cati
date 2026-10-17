# Pinned toolchain for reproducible tests and golden-image generation.
# go.mod requires "go 1.26.0" — this image pins that exact patch so
# stdlib decoders (e.g. image/jpeg) can't drift golden renders between
# contributors' machines. Regenerate goldens only from inside this image.
FROM golang:1.26.0-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

ARG HARNEZ_VERSION=v0.1.31-0.20261017082349-aeff3cee56cf
RUN go install ubunatic.com/harnez/cmd/harnez@${HARNEZ_VERSION}

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY scripts/container-test.sh /usr/local/bin/container-test
RUN chmod +x /usr/local/bin/container-test

ENV CATI_CONTAINER_TEST=1 GOCACHE=/tmp/cati-gocache HOME=/tmp
CMD ["/usr/local/bin/container-test"]
