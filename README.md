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

Dockerfile 没有使用 `# syntax=docker/dockerfile:1`，避免构建开始前还要从 docker.io 拉取 dockerfile frontend；自带 frontend（Docker 20.10+ / BuildKit 0.8+）已支持 `RUN --mount=type=cache`。

CI 构建时 docker.io 通常不可达，而 buildx 的 `docker-container` builder 不会继承 runner 上 dockerd 的 `registry-mirrors`。因此 workflow 会：

1. 依次探测 `DOCKER_MIRRORS` 里的镜像站（`curl https://<mirror>/v2/` 返回 200/401 视为可用）；
2. 把基础镜像直接改写成该镜像站的完整地址（如 `47k76iq5.mirror.aliyuncs.com/library/node:24-alpine`），BuildKit 因此完全不需要访问 docker.io；
3. 同时把可用镜像站写进 `.buildx/buildkitd.toml`，通过 `setup-buildx-action` 的 `config` 注入，作为兜底。

构建日志会打印每个镜像站的探测结果和最终使用的基础镜像名，便于排查。本地访问不了 docker.io 时，也可以手动指定基础镜像：

```bash
docker build \
  --build-arg NODE_IMAGE=docker.1ms.run/library/node:24-alpine \
  --build-arg GO_IMAGE=docker.1ms.run/library/golang:1.27-alpine \
  --build-arg RUNTIME_IMAGE=docker.1ms.run/library/alpine:3.22 \
  -t go-messager:local .
```

CI 中可通过仓库「Settings → Variables」覆盖以下变量：

| 变量             | 默认值                                                                    | 说明                                                                                   |
| ---------------- | ------------------------------------------------------------------------- | -------------------------------------------------------------------------------------- |
| `DOCKER_MIRRORS` | `47k76iq5.mirror.aliyuncs.com,docker.1ms.run,docker.1panel.live`          | docker.io 镜像加速地址，逗号分隔；按顺序探测，第一个可用的用于改写基础镜像并写入 buildkitd.toml |
| `NODE_IMAGE`     | 留空                                                                      | 显式设置时原样使用；留空则自动拼成 `<mirror>/library/node:24-alpine`，无可用镜像站时用 `node:24-alpine` |
| `GO_IMAGE`       | 留空                                                                      | 同上，默认为 `golang:1.27-alpine`                                                       |
| `RUNTIME_IMAGE`  | 留空                                                                      | 同上，默认为 `alpine:3.22`                                                              |

> 注意：`DOCKER_MIRRORS` 默认第一项就是当前 runner 上 dockerd 正在使用的阿里云加速地址；换机器部署时请改成自己环境里可用的地址（置空则不做探测、直接使用 docker.io）。

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
