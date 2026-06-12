#!/usr/bin/env bash
# 测试 Docker 多平台构建（linux/amd64、386、arm64、arm/v7、arm/v6）

set -e
cd "$(dirname "$0")"

PLATFORMS="linux/amd64,linux/386,linux/arm64/v8,linux/arm/v7,linux/arm/v6"
echo "==> 正在测试 Docker 构建平台：$PLATFORMS"
docker buildx build \
  --platform "$PLATFORMS" \
  -f Dockerfile \
  --progress=plain \
  . 2>&1 | tee docker-build-test.log

echo "==> 完成。完整输出请查看 docker-build-test.log。"
