# Go Messager

v0.1.2

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

JWT 相关环境变量优先级高于配置文件。Docker 部署时配置文件位于 `/conf/app.ini`，数据库位于 `/data/data.db`，持久化挂载 `/data`：

```bash
docker run -d --name go-messager -p 8080:8080 \
  -v go-messager-data:/data \
  -e JWT_SECRET='请替换为随机长字符串' \
  ghcr.io/<owner>/go-messager:latest
```

从旧版本升级时，把原来的 `data.db` 移动到 `data/` 目录：

```bash
mkdir -p data
mv data.db data/data.db
```

## 镜像构建

CI 在推送 `vX.Y.Z` 标签时构建并推送镜像（见 `.gitea/workflows/Build-Docker.yml`）。本地构建：

```bash
docker build -t go-messager:local .
```

Dockerfile 没有使用 `# syntax=docker/dockerfile:1`，避免构建开始前还要从 docker.io 拉取 dockerfile frontend；自带 frontend（Docker 20.10+ / BuildKit 0.8+）已支持 `RUN --mount=type=cache`。访问不了 docker.io 时，可把基础镜像指向镜像加速地址：

```bash
docker build \
  --build-arg NODE_IMAGE=docker.1ms.run/library/node:24-alpine \
  --build-arg GO_IMAGE=docker.1ms.run/library/golang:1.27-alpine \
  --build-arg RUNTIME_IMAGE=docker.1ms.run/library/alpine:3.22 \
  -t go-messager:local .
```

CI 中可通过仓库「Settings → Variables」覆盖以下变量：

| 变量            | 默认值                                                   | 说明                                                        |
| --------------- | -------------------------------------------------------- | ----------------------------------------------------------- |
| `DOCKER_MIRRORS` | `docker.1ms.run,docker.m.daocloud.io,docker.1panel.live` | 写入 `buildkitd.toml` 的 docker.io 镜像加速地址，逗号分隔；置空则直连 docker.io |
| `NODE_IMAGE`    | `node:24-alpine`                                         | 前端构建阶段基础镜像                                        |
| `GO_IMAGE`      | `golang:1.27-alpine`                                     | 后端构建阶段基础镜像                                        |
| `RUNTIME_IMAGE` | `alpine:3.22`                                            | 运行阶段基础镜像                                            |

> buildx 的 `docker-container` builder 不会继承 runner 上 dockerd 的 `registry-mirrors`，因此 CI 会把 `DOCKER_MIRRORS` 生成到 `.buildx/buildkitd.toml` 并通过 `setup-buildx-action` 的 `config` 注入。

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
