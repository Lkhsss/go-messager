# Go Messager

v0.1.4

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

### Docker 部署注意：`/data` 权限

镜像内以非 root 的 `app` 用户运行（Alpine 系统用户），而 bind mount 进来的宿主目录通常属于 `root`。
SQLite 既要能读写库文件，也要能在同目录创建 journal/WAL 文件，权限不对时启动会直接 panic：

```text
panic: 数据库文件[/data/data.db]不可写：open /data/data.db: permission denied
main.main()
        go-messager/main.go:43
```

完整报错见第一行（`docker compose logs --no-color go-messager | head -5`）。处理方式任选其一：

1. 把宿主目录属主改成容器内运行用户（推荐）：

   ```bash
   # 查询镜像内 app 用户的 uid/gid
   docker compose run --rm --entrypoint id go-messager app
   # 按上面的 uid/gid 改属主，例如都是 100
   sudo chown -R 100:100 /data/go_messager/data
   docker compose up -d --force-recreate go-messager
   ```

2. 改用 named volume：`volumes: ["go-messager-data:/data"]`。首次创建时 Docker 会沿用镜像内 `/data` 的属主，无需额外 chown。

3. 临时以 root 运行：在 compose 里加 `user: "0:0"`（不推荐，容器进程会以 root 身份写宿主目录）。

> 从旧版本（以 root 运行）升级到当前版本时，`/data/data.db` 可能仍是 `root:root`，同样需要执行上面的 chown。

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

npm / pnpm 的依赖默认走 `https://registry.npmmirror.com`（Dockerfile 里的 `ARG NPM_REGISTRY`），需要官方源或私有源时：

```bash
docker build --build-arg NPM_REGISTRY=https://registry.npmjs.org -t go-messager:local .
```

CI 中可通过仓库「Settings → Variables」覆盖以下变量：

| 变量             | 默认值                                                                    | 说明                                                                                   |
| ---------------- | ------------------------------------------------------------------------- | -------------------------------------------------------------------------------------- |
| `DOCKER_MIRRORS` | `docker.1ms.run`                                                          | docker.io 镜像加速地址，逗号分隔；按顺序探测，第一个可用的用于改写基础镜像并写入 buildkitd.toml |
| `NODE_IMAGE`     | 留空                                                                      | 显式设置时原样使用；留空则自动拼成 `<mirror>/library/node:24-alpine`，无可用镜像站时用 `node:24-alpine` |
| `GO_IMAGE`       | 留空                                                                      | 同上，默认为 `golang:1.27-alpine`                                                       |
| `RUNTIME_IMAGE`  | 留空                                                                      | 同上，默认为 `alpine:3.22`                                                              |
| `NPM_REGISTRY`   | `https://registry.npmmirror.com`                                          | npm / pnpm 依赖镜像源，同时用于 CI 安装依赖和镜像构建中的 pnpm 安装                       |
| `DOCKER_CACHE_TYPE` | `registry`                                                             | Docker 层缓存后端：`registry`（缓存推到镜像仓库标签）或 `gha`（走 Actions 缓存 API）      |
| `DOCKER_CACHE_REF`  | `buildcache`                                                           | `registry` 模式下的缓存标签名（`<镜像>:buildcache`），`gha` 模式下的缓存作用域            |

> 注意：`DOCKER_MIRRORS` 默认第一项就是当前 runner 上 dockerd 正在使用的阿里云加速地址；换机器部署时请改成自己环境里可用的地址（置空则不做探测、直接使用 docker.io）。

### CI 缓存

`.gitea/workflows/Build-Docker.yml` 里一共用了三层缓存，缓存 key 都由锁文件的哈希决定：

| 缓存 | 存放位置 | key 依据 | 命中后的表现 |
| --- | --- | --- | --- |
| Go 依赖 + 编译产物 | `actions/setup-go` 内置缓存（`GOMODCACHE` + `GOCACHE`） | `go.sum` | 日志出现 `Cache restored from key: setup-go-...` |
| 前端依赖 | `pnpm/setup` 缓存 pnpm store | `template/pnpm-lock.yaml` | 日志出现 `Cache restored from key: ...`，`pnpm install` 走 `--prefer-offline` |
| Docker 构建层 | buildx 缓存，默认推到 `<镜像>:buildcache` 标签（`mode=max`） | 每条指令 + 构建上下文哈希 | 构建输出里耗时步骤显示 `CACHED` |

几个排查用的命令（在 workflow 里也打印了这些信息）：

```bash
go env GOMODCACHE GOCACHE      # Go 两层缓存目录
( cd template && pnpm store path )   # pnpm store 目录
docker buildx imagetools inspect <镜像>:buildcache   # 查看 Docker 层缓存是否已推送
```

依赖源由 `NPM_REGISTRY` 控制（默认 `https://registry.npmmirror.com`）。`pnpm-lock.yaml` 只记录 integrity 哈希、不含 registry 地址，
所以换镜像源不会让 `--frozen-lockfile` 失败；cache key 也不受影响，换源后首次仍可复用已有 store。

强制刷新缓存：改 `go.sum` / `pnpm-lock.yaml` 会自然换 key；Docker 层缓存想重来，换一个 `DOCKER_CACHE_REF`（如 `buildcache2`）或删掉仓库里对应的 `buildcache` 标签即可。

### 在 Gitea 上启用 Actions 缓存

上面 Go / pnpm 的缓存都走 Actions 的缓存 API（`actions/cache` 那套），**服务端在 runner 侧，不是仓库文件能决定的**。
老版本 `act_runner` 不提供缓存服务，此时日志里只有 `Cache service url not found`，缓存静默失效但不会让构建失败；
新版 `gitea-runner` 自带缓存服务（cache service v2，同时兼容 `actions/upload-artifact` / `download-artifact` v4）。

单 runner：在 runner 的 `config.yaml`（可用 `./gitea-runner config init` 生成模板）里打开本地缓存目录，并把它持久化：

```yaml
cache:
  dir: /data/actcache        # 容器部署时把该目录挂成 volume，重建 runner 不丢缓存
  # retention: 168h          # 未使用缓存的保留时长，默认 168h
  # repo_size_limit: 10GB    # 单仓库缓存上限，默认 10GB，超出按最近最少使用淘汰
```

多 runner 共享缓存：把其中一个跑成 cache-server，其余指向它（`external_secret` 用 `openssl rand -hex 32` 生成，两边保持一致）：

```yaml
# cache-server.yaml
cache:
  dir: /data/actcache
  port: 8088
  external_secret: "<secret>"
```

```bash
gitea-runner -c cache-server.yaml cache-server
```

```yaml
# 其余 runner 的 config.yaml
cache:
  external_server: "http://<cache-server-host>:8088/"
  external_secret: "<secret>"
```

也可以让 runner 直接用 S3 / MinIO 存缓存（无需 cache-server）：

```yaml
cache:
  s3:
    endpoint: "http://minio:9000"
    region: "us-east-1"
    bucket: "runner-cache"
    prefix: "gitea-runner-cache"
    path_style: true
    access_key_file: "/run/secrets/cache-s3-access-key"
    secret_key_file: "/run/secrets/cache-s3-secret-key"
```

> S3 的过期清理不走 `cache.retention`，需要在桶上配生命周期规则，并让未完成的分片上传自动中止。

注意：缓存服务是 runner 侧的一个 HTTP 服务，**job 容器必须能访问到它**。同理，如果把 Docker 层缓存切到 `gha`
（`DOCKER_CACHE_TYPE=gha`），buildx 的构建容器也要能访问该地址；网络不通时保持默认的 `registry` 模式即可。

验证方式：连续重跑两次 workflow，第二次日志里出现 `Cache restored from key`，且依赖安装/编译耗时明显下降。

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
