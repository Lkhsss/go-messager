# Go Messager

v0.1.1

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
