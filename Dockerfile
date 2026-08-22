FROM golang:1.26.6-alpine AS backend-builder
WORKDIR /src/backend
ARG TARGETARCH
ARG TARGETVARIANT
ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"
ENV GOARCH=$TARGETARCH

RUN apk add --no-cache gcc musl-dev libc-dev make git wget unzip bash curl
ENV CC=gcc

RUN CRONET_ARCH="$TARGETARCH" && \
    CRONET_URL="https://github.com/SagerNet/cronet-go/releases/latest/download/libcronet-linux-${CRONET_ARCH}.so" && \
    wget -q -O /tmp/libcronet.so "$CRONET_URL" && chmod 755 /tmp/libcronet.so

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
      mkdir -p /tmp/s-ui-bin /tmp/xray && \
      wget -q -O /tmp/xray.zip "https://github.com/XTLS/Xray-core/releases/latest/download/${XRAY_ASSET}" && \
      unzip -q /tmp/xray.zip -d /tmp/xray && \
      cp /tmp/xray/xray /tmp/xray/geoip.dat /tmp/xray/geosite.dat /tmp/s-ui-bin/ && \
      chmod 755 /tmp/s-ui-bin/xray; \
    else \
      mkdir -p /tmp/s-ui-bin; \
    fi

COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN if [ "$TARGETARCH" = "arm" ]; then export GOARM=7; [ "$TARGETVARIANT" = "v6" ] && export GOARM=6; fi; \
    go build -ldflags="-w -s" \
      -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_naive_outbound,with_purego,with_tailscale" \
      -o /tmp/sui . && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-w -s" -o /tmp/sui-agent ./cmd/sui-agent

FROM alpine
ENV TZ=Asia/Shanghai
ENV SUI_API_LISTEN=0.0.0.0
ENV SUI_API_PORT=2097
WORKDIR /app
RUN apk add --no-cache --upgrade bash tzdata ca-certificates nftables
COPY --from=backend-builder /tmp/sui /tmp/sui-agent /tmp/libcronet.so /app/
COPY --from=backend-builder /tmp/s-ui-bin/ /app/bin/
COPY entrypoint.sh /app/
RUN chmod +x /app/entrypoint.sh
EXPOSE 2097
ENTRYPOINT ["./entrypoint.sh"]
