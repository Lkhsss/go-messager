# 注意：这里刻意不写 `# syntax=docker/dockerfile:1`。
# 该指令会让 BuildKit 在构建开始前先去 registry-1.docker.io 拉取 dockerfile
# frontend 镜像，在无法访问 docker.io 的网络里会直接超时失败。
# 本文件只用到 Dockerfile frontend 1.2+ 的稳定能力（RUN --mount=type=cache），
# Docker 20.10 / BuildKit 0.8 及以后版本自带的 frontend 即可支持，无需额外拉取。

# 基础镜像默认走 docker.io，可用 build-arg 指向镜像加速地址，例如：
#   --build-arg NODE_IMAGE=docker.1ms.run/library/node:24-alpine
#   --build-arg GO_IMAGE=docker.1ms.run/library/golang:1.27-alpine
#   --build-arg RUNTIME_IMAGE=docker.1ms.run/library/alpine:3.22
ARG NODE_IMAGE=node:24-alpine
ARG GO_IMAGE=golang:1.27-alpine
ARG RUNTIME_IMAGE=alpine:3.22

# ---------------------------------------------------------------------------
# 1. 前端构建：pnpm 安装依赖并执行 vue-tsc 类型检查 + vite 打包
#    使用 BUILDPLATFORM，多架构构建时前端只在当前 runner 架构上执行一次
# ---------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM ${NODE_IMAGE} AS frontend

ARG PNPM_VERSION=11

WORKDIR /src/template

# 不依赖 corepack（Node 24 之后 corepack 行为有变化），直接装指定大版本的 pnpm
RUN npm install --global pnpm@${PNPM_VERSION}

# 先拷贝清单文件，依赖不变时可以命中缓存
COPY template/package.json template/pnpm-lock.yaml ./
RUN --mount=type=cache,id=pnpm,target=/pnpm-store \
    pnpm install --frozen-lockfile --store-dir=/pnpm-store

COPY template/ ./
RUN pnpm run build

# ---------------------------------------------------------------------------
# 2. 后端构建：把前端 dist 放进 template/dist 后编译，main.go 通过 go:embed 打包
# ---------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS backend

ARG TARGETOS=linux
ARG TARGETARCH
# 可通过 --build-arg 覆盖为国内代理，例如 https://goproxy.cn,direct
ARG GOPROXY=https://proxy.golang.org,direct
ARG GOSUMDB=sum.golang.org

ENV GOPROXY=${GOPROXY} \
    GOSUMDB=${GOSUMDB}

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
COPY --from=frontend /src/template/dist ./template/dist

# glebarez/sqlite 是纯 Go 实现，可以关闭 CGO，产出静态二进制
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-$(go env GOARCH)} \
    go build -trimpath -ldflags="-s -w" -o /out/go-messager .

# ---------------------------------------------------------------------------
# 3. 运行镜像：非 root 运行，/data 用于持久化 SQLite 数据库 data.db
# ---------------------------------------------------------------------------
FROM ${RUNTIME_IMAGE} AS runtime

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app \
    && mkdir -p /data \
    && chown -R app:app /data

COPY --from=backend /out/go-messager /usr/local/bin/go-messager

# 配置文件从 ./conf/app.ini 读取；WORKDIR=/ 时对应 /conf/app.ini
COPY conf/ /conf/

# 默认数据库路径 ./data/data.db 对应 /data/data.db，与上一版保持一致
WORKDIR /

# GIN_MODE=release 时 main.go 才会挂载嵌入的前端静态资源
ENV GIN_MODE=release \
    TZ=Asia/Shanghai

EXPOSE 8080
VOLUME ["/data"]

USER app

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/api/uptime || exit 1

ENTRYPOINT ["go-messager"]
