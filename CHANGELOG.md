# 更新日志

## v0.13.0

### 新增

- 正文 `#标签` 自动识别：发布/编辑文章时从 Markdown 正文提取标签并自动关联，新标签进入待审核状态
  - 正文中渲染为行内彩色药丸（`display: inline`，不改变行高；标签之间与文字之间保留间隙）
  - 编辑页实时解析正文标签并显示在标签区下方，点击可定位到正文；预览区同步显示药丸
  - 标签数量不设上限（手选与自动提取），列表卡片超出 8 个以 `+N` 收起
- 标签审核：新增 `GET /api/admin/tags` 与 `PATCH /api/admin/tags/:id/review`
  - 未审核标签不进入文章下方标签区与首页筛选；正文中未审核标签使用默认灰色，悬浮提示「标签待审核」
  - 管理员标签管理支持审核通过 / 取消审核、待审核筛选与计数
  - 升级时历史标签一次性标记为已审核（`settings` 表 `tag_review_migrated` 标记）

### 修复

- 连续标签写法 `#TEST#自动标签#ZZZ` 现在会识别为多个标签；`##标题`、`a#b` 以及链接/代码中的 `#` 仍不会误识别

## v0.12.0

### 新增

- 登录态滑动续期：认证通过且 token 剩余有效期不足 `ttl/4`（最多 12 小时，最少 30 秒）时，服务端换发新 token 并通过 `X-Auth-Token`、`X-Auth-Expires-At` 响应头下发，前端自动更新本地登录态
  - `jwt.ttl` 语义变为「不活跃容忍时长」：配置为 `168h` 即「7 天内访问过就保持登录」
  - 前端收到 401 时先比较失败请求携带的 token 与当前 token，避免多标签或续期竞态误清刚续上的新 token
  - 多标签页通过 `storage` 事件同步登录态

## v0.11.0

### 新增

- 防护自动化灌水与账号农场：
  - `[limits]` 配置每分钟发文/评论/互动频率、每日配额、每 IP 注册上限、密码/用户名约束和内容去重窗口
  - 新账号（注册不足 `new_account_hours` 且未可信）默认每天 5 篇文章、20 条评论；标记可信后解除
  - 同一用户相同正文（忽略空白）在窗口内返回 `409 DUPLICATE_POST`；评论使用 10 分钟窗口
  - 请求体默认上限 1MiB，超出返回 `413 BODY_TOO_LARGE`
  - 验证码 ticket 绑定签发 IP，跨 IP 不可复用
  - 登录增加按用户名的跨 IP 失败上限；不存在的账号也执行一次 bcrypt，降低时序枚举风险
- 数据库索引：`posts(user)`、`posts(user, root_id)`、`posts(user, body_hash)`、`posts(created_at)`、`post_tags(post_id)`、`reactions(user_id)`，并为首页增加 `posts(root_id, deleted_at, id)`

### 变更

- 首页/评论列表按 `id` 排序，避免大表临时排序
- 用户统计、管理员用户聚合、统计曲线和热力图改为 SQL 聚合，不再把全量文章加载进 Go
- 启动时反应计数校准只更新有脏计数或受反应影响的行，避免每次启动重写整张 posts 表
- 每日任务已完成时跳过写事务，`ensureDailyProgress` 改为先查后建，避免 `ON CONFLICT DO NOTHING` 消耗自增序列
- 清理和「删除未可信用户」的文章删除改为分批执行，降低单条 SQL 变量数量和峰值开销
- 前端注册/初始化/管理员改密同步要求密码至少 8 字节、用户名至少 2 字符

## v0.9.0

### 新增

- 用户管理支持「可信用户」标记：管理员可把用户设为/取消可信，列表显示「可信 / 未可信」徽章
  - 首个管理员注册时直接可信，其余新注册用户默认不可信，为后续邮箱验证铺垫
  - `PATCH /api/admin/users/:id/trusted`（body `{"trusted": true|false}`），管理员账号不允许修改
- 用户管理新增「清理未可信用户」：`DELETE /api/admin/users/untrusted` 硬删除所有未标记可信的普通用户
  - 同时删除这些账号的文章、评论与回复、点赞等反应和每日任务进度，并重新校准标签使用次数与反应统计
  - 账号彻底删除，用户名可以再次注册；管理员账号不受影响
  - 升级说明：已有用户升级后默认也是「未可信」，批量清理前请先标记需要保留的账号
- 新增注册开关 `[register] enabled`（默认 `true`）：关闭后除首个管理员初始化外不再允许注册，`POST /api/register` 返回 `403 REGISTER_DISABLED`，登录页隐藏注册入口，修改配置后需重启生效
- SQLite 并发调优：默认开启 WAL、`busy_timeout=5s`、`_txlock=immediate`，连接池默认 1，缓解被刷或并发写导致的 `database is locked (5) (SQLITE_BUSY)`；新增 `[database] busy_timeout`、`[database] max_open_conns` 与 `[server] trusted_proxies` 配置
- 登录限流：每 IP 20 次/分钟，同一 IP + 用户名 15 分钟内 5 次失败后临时锁定，返回 `429 LOGIN_RATE_LIMITED` / `LOGIN_LOCKED`，登录成功清零
- 管理页新增「清理」标签页：统计并清理已删除文章、已删除评论、孤立评论/反应/标签关联/每日任务
  - `GET /api/admin/cleanup` 返回各分类数量、估算占用与数据库真实大小/空闲页，前端用饼图展示各分类占比与空间占用
  - `DELETE /api/admin/cleanup/:key` 与 `DELETE /api/admin/cleanup` 硬删除对应数据，并重新校准标签使用次数与反应统计
  - `POST /api/admin/cleanup/vacuum` 执行 `VACUUM` 回收空闲页
  - 修复「清理未可信用户」硬删除文章时遗留 `post_tags` 与评论反应的问题

## v0.6.1 2026-11-11

### 新增

- 写文章自动草稿：输入时静默保存到浏览器本地（按用户隔离），未发布就离开后再次进入「写文章」自动恢复标题、正文和标签；发布成功自动清除，并提供「清除草稿」确认弹窗

## v0.6.0 2026-11-11

### 新增

- 注册人机验证：新用户注册前需在独立页面 `/verify` 完成图形验证码
  - `GET /api/captcha` 返回 PNG 验证码（标准库绘制，2 分钟有效），`POST /api/captcha/verify` 通过后签发 5 分钟有效的一次性 ticket
  - 注册接口要求携带 `captcha_ticket`，缺失/失效返回 `CAPTCHA_REQUIRED`；首个管理员初始化时免验证
  - 同一道题最多 5 次错误机会，生成 30 次/分钟/IP、校验 20 次/分钟/IP 限流
  - 新增配置 `[captcha] enabled`（默认 `true`），维护模式白名单放行验证码接口
- 用户管理新增「清空文章」：一键软删除指定用户的全部文章及其下的评论和回复，账号保留，标签使用次数自动校准

## v0.5.1 2026-11-11

### 新增

- 维护模式：管理页新增「维护模式」标签页，可设置标题和可选正文
  - 维护页标题与「前方禁行」以 5 秒为周期交替滚动；没有正文时只显示居中放大的标题
  - 开启后普通用户无法登录、注册和访问任何接口，仅管理员可用
  - 管理员在管理页手动关闭后普通用户立即恢复，维护页每 15 秒自动检测
  - 状态保存在 `settings` 表（key 为 `maintenance`），重启后保留
- 维护期间白名单：`GET /api/status`、`GET /api/uptime`、`POST /api/authorize`（管理员登录以恢复站点）、`POST /api/register`（仅全新库初始化管理员时放行）

### 变更

- README 新增「维护模式」章节
- 维护页标题改用 daisyUI `text-rotate`，并加大行高避免字形下缘被裁切

### 修复

- 首页与「我的文章」卡片的长标题、副标题和摘要不再溢出容器
  - 标题里的 flex item 加 `min-w-0`，解除 `min-width: auto` 导致的无法收缩
  - 长串无空格文本（如连续的 `+3+3…`）用 `break-words` 在任意位置折行

## v0.4.3 2026-11-11

### 修复

- 跨天保持登录（token 未过期）时，访问 `GET /api/me` 自动补发当日登录经验，不再需要重新登录
  - 新增 `dailyLoginPending` 预检，当天已完成时只读一次数据库、不开写事务
  - 补充跨天补发与同日不重复发放的测试

## v0.4.2 2026-11-11

### 变更

- 精简 Docker 构建工作流：去掉镜像站探测、构建缓存导出和测试 job，只保留构建与推送
- 镜像标签规则：推送 tag 时发布版本标签（如 `0.5.0`）和 `latest`；手动触发只发布 `latest`
- Dockerfile 移除基础镜像 ARG（`NODE_IMAGE` / `GO_IMAGE` / `RUNTIME_IMAGE`），直接使用官方基础镜像
- README 精简「镜像构建」章节与 CI 变量说明

## v0.4.1 2026-11-11

### 变更

- CI：Go 与 pnpm 依赖启用缓存，测试 job 更名为 `setup`
- CI：改用 `pnpm/setup` 安装 pnpm 11 和 Node 26，并固定 npmmirror 源
- 新增 VS Code 扩展推荐 `.vscode/extensions.json`

## v0.4.0 2026-11-10

### 新增

- 每日经验系统：登录、写文章、评论、互动按天发放经验，并通过接口响应返回本次获得的 XP
  - 登录：每天首次登录 +1
  - 写文章：发布不少于 200 字的文章 +5，每天一次
  - 评论：发表不少于 100 字的评论 +3，每天一次
  - 互动：点赞、点踩、疑问和表情反应每次 +1，每天最多 5 次
- 等级系统：累计经验按 100 / 200 / 300… 递增换算等级，文章列表与个人名片显示等级
- 新增 `GET /api/me/experience`（等级进度与每日任务状态）和 `DailyTaskProgress` 表
- 前端新增经验 toast（获得经验、升级提示）和「我的」页等级经验面板

## v0.3.0 2026-11-09

### 新增

- 站点访问模式 `access.mode`，支持 `private`（默认）和 `public`
  - `private`：所有页面和操作都必须登录
  - `public`：游客可浏览首页、文章详情、评论列表、标签列表和排行榜；发帖、编辑、删除、点赞、评论等写操作仍需登录
  - 游客触发写操作时显示 401 提示页或登录弹窗，登录页新增「先逛逛首页」入口
- 新增 `conf/app.min.ini` 最小配置，`conf/app.example.ini` 作为完整示例配置
- README 新增「界面预览」，包含首页、文章详情、排行榜、管理后台四张截图

### 变更

- `server.mode` 留空时默认 `release`，此前会回落到 Gin 默认的 `debug`
- `conf/app.ini` 加入 `.gitignore` 和 `.dockerignore`，本地配置不再提交，也不会进入构建上下文
- Dockerfile 只复制 `conf/app.min.ini` 为 `/conf/app.ini`
- `docs/` 加入 `.dockerignore`，截图不进入镜像构建上下文

## v0.2.3 2026-11-08

### 新增

- 404 页面，并注册 catch-all 路由

### 修复

- 主内容区布局改为 flex column，修复页面高度异常

### 其他

- 本地 `app.ini` 切换为 debug 模式，注释掉 jwt 默认值
- 精简 CI 工作流与配置文档中的注释
- README 配置表格格式和 TODO 列表更新

## v0.2.2 2026-11-08

### 新增

- `DOCKER_CACHE_TYPE=none`，跳过构建缓存导出，仅使用 runner 本地缓存

### 修复

- Docker 引擎未启用 containerd 镜像存储时，工作流通过 `docker info` 自动检测并告警，回退到本地缓存，不再让构建失败

## v0.2.1 2026-11-08

### 新增

- 全局 gzip 中间件（`gin-contrib/gzip`）
  - 仅在请求带 `Accept-Encoding: gzip` 时压缩，并返回 `Vary: Accept-Encoding`
  - 跳过 `/api/avatar` 和 png/jpg/webp/woff2 等已压缩内容

### 修复

- 移除 `gzip.WithMinLength`：`gin-contrib/gzip v1.2.9` 会让小响应出现「声明 gzip、正文明文」，导致浏览器解码失败、页面白屏
- 补充小响应压缩测试，包括覆盖 `c.Data`/`Content-Length` 路径的真实服务测试

### 文档

- README 说明 gzip 中间件行为，压缩级别和排除规则见 `gzipCompression()`

## v0.1.4 2026-11-07

### 新增

- 打开数据库前探测库文件和目录可写性，提前暴露容器内 bind mount 权限问题
- README 补充 `/data` 权限排查与修复说明

### 变更

- CI 简化为只构建 runner 本机架构，移除 QEMU 和多架构构建
- 优化构建缓存与镜像源配置，Go 依赖使用 `go.sum` 作为缓存 key

## v0.1.3 2026-11-07

### 变更

- CI 依次探测 `DOCKER_MIRRORS` 中的镜像站，自动改写基础镜像地址，base image 拉取不再超时
- 默认镜像站精简为 `docker.1ms.run`

## v0.1.2 2026-11-07

### 变更

- 更新 Docker 构建流水线

## v0.1.1 2026-11-07

### 新增

- Gitea Actions 工作流 `.gitea/workflows/Build-Docker.yml`，推送 `vX.Y.Z` 标签或手动触发时构建并推送镜像

### 变更

- Dockerfile 支持通过 `--build-arg` 配置 `GOPROXY` 和 `GOSUMDB`

## v0.1.0 2026-11-07

首个版本。

### 后端

- 基于 Gin 的 API，统一响应结构和分页元信息
- SQLite（`glebarez/sqlite`，纯 Go 实现，无需 CGO），GORM 自动建表
- 用户注册/登录，bcrypt 密码哈希，JWT 鉴权，首个用户自动成为管理员
- 文章增删改查、软删除、分页和标签过滤（包含/排除）
- 评论与回复（`root_id` + `parent_id` 评论树）
- 点赞/点踩/疑问和 GitHub 风格表情反应，作者反应总计
- 文章私有可见性控制，管理员可隐藏文章
- 用户头像上传（data URL 存储）与公开头像接口
- 标签管理：创建、编辑颜色、删除并清理关联，使用次数统计
- 管理员功能：用户管理（封禁/解封/删除/改密）、全站统计、数据图表
- 文章标题/副标题/正文长度校验，Markdown 摘要和字数统计
- INI 配置（`conf/app.ini`），支持环境变量回退
- `/api/uptime` 返回启动时刻和运行时长

### 前端

- Vue 3 + Vite + Tailwind CSS + daisyUI
- 登录、初始化、首页、文章详情/编辑、我的文章、排行榜、数据、设置、管理后台等页面
- Markdown 渲染（markdown-it + DOMPurify）与代码高亮（highlight.js）
- 标签筛选组件、反应栏、3D 个人名片、运行时长倒计时、贡献热力图
- 响应式侧边栏布局和 SPA 路由守卫

### 部署

- Dockerfile 多阶段构建，前端产物通过 `go:embed` 打进二进制
- 运行镜像以非 root 用户运行，`/data` 持久化 SQLite 数据库
- 镜像内前端静态资源与 SPA 兜底路由

