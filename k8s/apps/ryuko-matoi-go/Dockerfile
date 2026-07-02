ARG GO_VERSION=1.25.5
ARG ALPINE_VERSION=3.22

FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS builder

WORKDIR /src

RUN apk add --no-cache \
	build-base \
	libwebp-dev \
	pkgconfig

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=1 go build -o /out/ryuko-matoi ./cmd/bot

FROM alpine:${ALPINE_VERSION}

ENV APP_HOME=/app

RUN apk add --no-cache \
	ca-certificates \
	ffmpeg \
	font-liberation \
	libwebp \
	python3 \
	py3-pip \
	tzdata \
	tesseract-ocr \
	tesseract-ocr-data-eng

RUN python3 -m venv /opt/yt-dlp-venv \
	&& /opt/yt-dlp-venv/bin/pip install --no-cache-dir --upgrade pip yt-dlp \
	&& ln -sf /opt/yt-dlp-venv/bin/yt-dlp /usr/local/bin/yt-dlp

WORKDIR ${APP_HOME}

COPY --from=builder /out/ryuko-matoi /usr/local/bin/ryuko-matoi
COPY fonts ./fonts

RUN mkdir -p /app/data /app/tmp

VOLUME ["/app/data"]

CMD ["ryuko-matoi"]
