FROM --platform=$BUILDPLATFORM node:alpine AS front-builder
WORKDIR /app
COPY frontend/ ./
RUN npm ci && npm run build

FROM golang:1.26.5-alpine AS backend-builder
WORKDIR /app
ARG TARGETARCH
ARG TARGETVARIANT
ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"
ENV GOARCH=$TARGETARCH

RUN apk update && apk add --no-cache \
    gcc \
    musl-dev \
    libc-dev \
    make \
    git \
    wget \
    unzip \
    bash \
    curl

ENV CC=gcc

RUN CRONET_ARCH="$TARGETARCH" && \
    CRONET_URL="https://github.com/SagerNet/cronet-go/releases/latest/download/libcronet-linux-${CRONET_ARCH}.so"; \
    echo "Downloading $CRONET_URL" && \
    wget -q -O ./libcronet.so "$CRONET_URL" && \
    chmod 755 ./libcronet.so

RUN XRAY_ASSET="" && \
    case "$TARGETARCH/$TARGETVARIANT" in \
      amd64/*) XRAY_ASSET="Xray-linux-64.zip" ;; \
      386/*) XRAY_ASSET="Xray-linux-32.zip" ;; \
      arm64/*) XRAY_ASSET="Xray-linux-arm64-v8a.zip" ;; \
      arm/v5) XRAY_ASSET="Xray-linux-arm32-v5.zip" ;; \
      arm/v6) XRAY_ASSET="Xray-linux-arm32-v6.zip" ;; \
      arm/*) XRAY_ASSET="Xray-linux-arm32-v7a.zip" ;; \
      s390x/*) XRAY_ASSET="Xray-linux-s390x.zip" ;; \
    esac && \
    if [ -n "$XRAY_ASSET" ]; then \
      mkdir -p /app/bin /tmp/xray && \
      wget -q -O /tmp/xray.zip "https://github.com/XTLS/Xray-core/releases/latest/download/${XRAY_ASSET}" && \
      unzip -q /tmp/xray.zip -d /tmp/xray && \
      cp /tmp/xray/xray /app/bin/xray && \
      chmod 755 /app/bin/xray && \
      cp /tmp/xray/geoip.dat /tmp/xray/geosite.dat /app/bin/ && \
      rm -rf /tmp/xray /tmp/xray.zip; \
    else \
      echo "No Xray-core asset mapping for $TARGETARCH/$TARGETVARIANT"; \
    fi

COPY . .
COPY --from=front-builder /app/dist/ /app/web/html/

RUN if [ "$TARGETARCH" = "arm" ]; then export GOARM=7; [ "$TARGETVARIANT" = "v6" ] && export GOARM=6; fi; \
    go build -ldflags="-w -s" \
    -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_naive_outbound,with_purego,with_tailscale" \
    -o sui main.go

FROM alpine
ENV TZ=Asia/Shanghai
WORKDIR /app
RUN set -ex && apk add --no-cache --upgrade bash tzdata ca-certificates nftables
COPY --from=backend-builder /app/sui /app/libcronet.so /app/
COPY --from=backend-builder /app/bin/ /app/bin/
COPY entrypoint.sh /app/
RUN chmod +x /app/entrypoint.sh
ENTRYPOINT [ "./entrypoint.sh" ]
