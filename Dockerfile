# 不写 `# syntax=docker/dockerfile:1`，避免构建前额外拉取 dockerfile frontend。
# 当前写法兼容 Docker 20.10 / BuildKit 0.8+。

# 基础镜像和 npm 源可通过 build-arg 覆盖：
#   --build-arg NODE_IMAGE=docker.1ms.run/library/node:24-alpine
#   --build-arg GO_IMAGE=docker.1ms.run/library/golang:1.27-alpine
#   --build-arg RUNTIME_IMAGE=docker.1ms.run/library/alpine:3.22
#   --build-arg NPM_REGISTRY=https://registry.npmjs.org
ARG NODE_IMAGE=node:24-alpine
ARG GO_IMAGE=golang:1.27-alpine
ARG RUNTIME_IMAGE=alpine:3.22

# 1. 前端构建
FROM --platform=$BUILDPLATFORM ${NODE_IMAGE} AS frontend

ARG PNPM_VERSION=11
ARG NPM_REGISTRY=https://registry.npmmirror.com
# npm 和 pnpm 11+ 读取不同的环境变量，两个都设。
ENV npm_config_registry=${NPM_REGISTRY} \
    pnpm_config_registry=${NPM_REGISTRY}

WORKDIR /src/template

# 直接用 npm 安装 pnpm，并缓存 npm 下载目录。
RUN --mount=type=cache,id=npm,target=/root/.npm \
    npm install --global pnpm@${PNPM_VERSION}

# 先拷贝清单文件，依赖不变时可命中缓存。
COPY template/package.json template/pnpm-lock.yaml ./
RUN --mount=type=cache,id=pnpm,target=/pnpm-store \
    pnpm install --frozen-lockfile --prefer-offline --registry=${NPM_REGISTRY} --store-dir=/pnpm-store

COPY template/ ./
RUN pnpm run build

# 2. 后端构建
FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS backend

ARG TARGETOS=linux
ARG TARGETARCH
# 可用 build-arg 覆盖 Go 代理。
ARG GOPROXY=https://proxy.golang.org,direct
ARG GOSUMDB=sum.golang.org

ENV GOPROXY=${GOPROXY} \
    GOSUMDB=${GOSUMDB}

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
COPY --from=frontend /src/template/dist ./template/dist

# 纯 Go SQLite，关闭 CGO。
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-$(go env GOARCH)} \
    go build -trimpath -ldflags="-s -w" -o /out/go-messager .

# 3. 运行镜像
FROM ${RUNTIME_IMAGE} AS runtime

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app \
    && mkdir -p /data \
    && chown -R app:app /data

COPY --from=backend /out/go-messager /usr/local/bin/go-messager

# 配置文件在 /conf/app.ini。
COPY conf/ /conf/

# 数据库默认 /data/data.db。
WORKDIR /

# release 模式才会挂载嵌入的前端。
ENV GIN_MODE=release \
    TZ=Asia/Shanghai

EXPOSE 8080
VOLUME ["/data"]

USER app

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/api/uptime || exit 1

ENTRYPOINT ["go-messager"]
