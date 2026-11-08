# Go Messager

v0.2.3

Go语言制作的极简论坛

## 配置

默认读取进程工作目录下的 `conf/app.ini`。首次使用可复制示例：

```bash
cp conf/app.example.ini conf/app.ini
```

Windows PowerShell：

```powershell
Copy-Item conf/app.example.ini conf/app.ini
```

默认值：

| 配置项          | 默认值           | 说明                                              |
| --------------- | ---------------- | ------------------------------------------------- |
| `server.host`   | 空               | debug 监听 `127.0.0.1`，release 监听 `0.0.0.0`    |
| `server.port`   | `8080`           | HTTP 端口                                         |
| `server.mode`   | 空               | `debug` / `release` / `test`，留空使用 `GIN_MODE` |
| `database.path` | `./data/data.db` | SQLite 文件路径，目录自动创建                     |
| `jwt.secret`    | 空               | 回退到 `JWT_SECRET` 或数据库自动生成              |
| `jwt.ttl`       | `24h`            | `JWT_TTL` 环境变量优先                            |

后端默认开启 gzip 压缩（`gin-contrib/gzip`），对 API JSON 与嵌入的前端静态资源都生效：

- 仅当请求头带 `Accept-Encoding: gzip` 时压缩，并返回 `Vary: Accept-Encoding`；
- `/api/avatar` 以及 `.png`/`.jpg`/`.webp`/`.woff2` 等已压缩内容不重复压缩；
- 4xx/5xx 与无 body 的响应（204/304）不压缩；
- 不使用 `WithMinLength`：该选项在 `gin-contrib/gzip v1.2.9` 中会让 `c.Data` 的小响应出现「声明 gzip、body 明文」，浏览器解码失败。

压缩级别与排除规则在 `middleware.go` 的 `gzipCompression()` 中调整。

JWT 相关环境变量优先级高于配置文件。Docker 部署时配置文件位于 `/conf/app.ini`，数据库位于 `/data/data.db`，持久化挂载 `/data`：

```bash
docker run -d --name go-messager -p 8080:8080 \
  -v go-messager-data:/data \
  -e JWT_SECRET='请替换为随机长字符串' \
  ghcr.io/<owner>/go-messager:latest
```

## 镜像构建

CI 在推送 `vX.Y.Z` 标签时构建并推送镜像（见 `.gitea/workflows/Build-Docker.yml`）。本地构建：

```bash
docker build -t go-messager:local .
```

Dockerfile 没有使用 `# syntax=docker/dockerfile:1`，避免构建开始前还要从 docker.io 拉取 dockerfile frontend；自带 frontend（Docker 20.10+ / BuildKit 0.8+）已支持 `RUN --mount=type=cache`。

CI 不创建独立的 BuildKit 容器，也不使用 QEMU：直接调用 runner 上 Docker 引擎自带的 builder（`docker` driver），**只构建 runner 本机架构**（amd64 runner 产出 amd64 镜像）。链路更短，也不会再遇到自建 builder 里旧版 BuildKit 的会话/推送问题。

基础镜像仍走镜像加速，workflow 会：

1. 依次探测 `DOCKER_MIRRORS` 里的镜像站（`curl https://<mirror>/v2/` 返回 200/401 视为可用）；
2. 把可用镜像站拼进基础镜像地址（如 `docker.1ms.run/library/node:24-alpine`），docker.io 不可达时也能拉到。

构建日志会打印每个镜像站的探测结果和最终使用的基础镜像名，便于排查。本地访问不了 docker.io 时，也可以手动指定基础镜像：

```bash
docker build \
  --build-arg NODE_IMAGE=docker.1ms.run/library/node:24-alpine \
  --build-arg GO_IMAGE=docker.1ms.run/library/golang:1.27-alpine \
  --build-arg RUNTIME_IMAGE=docker.1ms.run/library/alpine:3.22 \
  -t go-messager:local .
```

npm / pnpm 的依赖默认走 `https://registry.npmmirror.com`（Dockerfile 里的 `ARG NPM_REGISTRY`），需要官方源或私有源时：

```bash
docker build --build-arg NPM_REGISTRY=https://registry.npmjs.org -t go-messager:local .
```

CI 中可通过仓库「Settings → Variables」覆盖以下变量：

| 变量                | 默认值                           | 说明                                                                                                                                                |
| ------------------- | -------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| `DOCKER_MIRRORS`    | `docker.1ms.run`                 | docker.io 镜像加速地址，逗号分隔；按顺序探测，第一个可用的用于改写基础镜像地址                                                                      |
| `NODE_IMAGE`        | 留空                             | 显式设置时原样使用；留空则自动拼成 `<mirror>/library/node:24-alpine`，无可用镜像站时用 `node:24-alpine`                                             |
| `GO_IMAGE`          | 留空                             | 同上，默认为 `golang:1.27-alpine`                                                                                                                   |
| `RUNTIME_IMAGE`     | 留空                             | 同上，默认为 `alpine:3.22`                                                                                                                          |
| `NPM_REGISTRY`      | `https://registry.npmmirror.com` | npm / pnpm 依赖镜像源，同时用于 CI 安装依赖和镜像构建中的 pnpm 安装                                                                                 |
| `DOCKER_CACHE_TYPE` | `registry`                       | Docker 层缓存后端：`registry`（缓存推到镜像仓库标签）、`gha`（走 Actions 缓存 API）或 `none`（只使用本地缓存）；driver 不支持导出时自动退回本地缓存 |
| `DOCKER_CACHE_REF`  | `buildcache`                     | `registry` 模式下的缓存标签名（`<镜像>:buildcache`），`gha` 模式下的缓存作用域                                                                      |

> 注意：`DOCKER_MIRRORS` 默认 `docker.1ms.run`，可覆盖成逗号分隔的多个地址；换机器部署时请改成自己环境里可用的地址（置空则不做探测、直接使用 docker.io）。

# TODO

1. [x] 卡片悬浮
2. [x] text-rotate
3. [x] countdown
4. [x] 排行(问号)榜
5. [x] emoji和表情
6. [x] Markdown支持
7. [x] 个人帖子界面
8. [x] 管理员隐藏或删除帖子
9. [x] 管理员查看数据统计：折线统计图，标签饼图，登陆人数
10. [x] md 短emoji
11. [x] 404界面
12. [ ] 公告和维护模式
13. [ ] 配置读取逻辑优化
14. [ ] 隐私模式和公开模式
15. [ ] 最小配置/docker配置

# FIXME

1. [x] 前端js分割
