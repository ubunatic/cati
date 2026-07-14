# Pinned toolchain for reproducible tests and golden-image generation.
# go.mod requires "go 1.25.0" — this image pins that exact patch so
# stdlib decoders (e.g. image/jpeg) can't drift golden renders between
# contributors' machines. Regenerate goldens only from inside this image.
FROM golang:1.25.0-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ENV GOCACHE=/tmp/cati-gocache
CMD ["make", "test"]
