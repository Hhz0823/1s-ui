#!/bin/sh
set -eu

(cd frontend && npm ci && npm run build)
tar -C frontend/dist -czf s-ui-frontend.tar.gz .

BUILD_TAGS="with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_naive_outbound,with_musl,badlinkname,tfogo_checklinkname0,with_tailscale"
(cd backend && go build -ldflags '-w -s -checklinkname=0 -extldflags "-Wl,-no_warn_duplicate_libraries"' -tags "$BUILD_TAGS" -o ../sui .)
(cd backend && CGO_ENABLED=0 go build -trimpath -ldflags '-w -s' -o ../sui-agent ./cmd/sui-agent)
