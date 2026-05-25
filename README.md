# NewAPI Guard

`NewAPI Guard` 是一个放在 `NewAPI` 前面的外挂守卫层，用来补齐以下几类能力：

- Discord 准入登录
- `/v1/*` 透明代理与风控
- `/api/user/checkin` 签到接管
- 白名单、封禁、日志、系统设置管理面板

它的目标不是替代 `NewAPI`，而是在**不改 NewAPI 源码**的前提下，补充 NewAPI 本身没有的控制能力。

## 项目结构

当前仓库已经包含可运行的后端与管理面板：

- Go 后端：透明代理、OAuth、签到、管理接口、SQLite 存储
- Vue 3 管理面板：控制台登录、用户管理、封禁、白名单、日志、系统设置
- Dockerfile：可直接构建 Guard 镜像
- `docker-compose.yml`：提供 `Guard + NewAPI` 同编排模板，并在最下方保留 `PostgreSQL / Redis` 作为 NewAPI 的常见外部依赖示例

## 核心设计

Guard 与 NewAPI 的关系如下：

```text
用户
  |
  v
反向代理 / 域名入口
  |
  +-- /v1/*             -> Guard
  +-- /api/user/checkin -> Guard
  +-- /guard/*          -> Guard
  +-- /*                -> NewAPI

Guard
  - Discord OAuth 准入
  - 请求风控
  - 签到拦截
  - 管理面板
  - SQLite 本地状态库

NewAPI
  - 原生前端
  - 原生用户体系
  - 原生管理员 API
  - 自定义 OAuth Provider 接入点
```

## 配置机制

这个项目当前有两套配置来源：

1. **环境变量**
   用于 Guard 进程启动参数，例如监听地址、数据库路径、管理员密码、NewAPI 地址等。
2. **SQLite `config` 表**
   用于后台设置页保存的大多数业务配置，例如 Discord OAuth 凭据、准入规则、UA 白名单、签到参数等。

需要特别注意：

- Guard 当前**不会自动读取 `.env` 文件**，代码里只调用 `os.Getenv`
- 如果你使用 Docker Compose，Compose 会把 `.env` 里的值注入容器，因此依然可以正常使用 `.env`
- `GUARD_ADMIN_PASSWORD` 如果不设置，管理面板默认无法登录
- `GUARD_NEWAPI_ADMIN_TOKEN` 如果不设置，Guard 仍可启动，但很多高级功能不可用

## 环境变量说明

根目录已经提供模板文件：[.env.example](./.env.example)

### Guard 至少要补的变量

这几个值建议优先设置：

- `GUARD_ADMIN_PASSWORD`
  控制面板登录密码
- `GUARD_NEWAPI_URL`
  Guard 访问 NewAPI 的内网地址
- `GUARD_NEWAPI_ADMIN_TOKEN`
  NewAPI 管理员令牌

### `GUARD_NEWAPI_ADMIN_TOKEN` 不填会怎样

如果这个值为空，以下功能会受限或不可用：

- 用户列表与用户搜索
- 创建用户
- 封禁 / 解封同步到 NewAPI
- 签到补发额度
- 代理层通过 token 反查用户身份

### 当前代码真正支持的 Guard 环境变量

- `GUARD_LISTEN_ADDR`
- `GUARD_DATA_DIR`
- `GUARD_DB_PATH`
- `GUARD_WEB_DIR`
- `GUARD_ADMIN_PASSWORD`
- `GUARD_NEWAPI_ADMIN_TOKEN`
- `GUARD_NEWAPI_URL`
- `GUARD_SESSION_TTL`
- `GUARD_TOKEN_CACHE_TTL`
- `GUARD_ENABLE_SCHEDULER`

补充说明：

- `GUARD_TOKEN_CACHE_TTL` 当前虽然会被读取，但代理层 token 缓存的实际过期时间仍然是代码里的固定值，后续如果需要可以再继续补齐这一项的真正生效逻辑

### 当前还不能通过环境变量设置的项

以下配置目前要到 Guard 后台里设置，或者提前写入 SQLite `config` 表：

- `public_base_url`
- `oauth_client_id`
- `oauth_client_secret`
- `oauth_provider_slug`
- `oauth_state_ttl_seconds`
- `oauth_code_ttl_seconds`
- `oauth_token_ttl_seconds`
- `discord_client_id`
- `discord_client_secret`
- `discord_guild_id`
- `discord_oauth_scopes`
- `discord_access_policy`
- `rpm_limit`
- `ua_ban_strikes`
- `allowed_ua`（默认留空）
- `checkin_quota`
- `checkin_threshold`

## Docker Compose 联合部署

根目录已提供模板文件：[docker-compose.yml](./docker-compose.yml)

这个编排把以下服务放在同一个网络里，方便容器间直接通信：

- `guard`
- `new-api`

其中 Guard 默认通过下面这个地址访问 NewAPI：

```text
http://new-api:3000
```

也就是说，只要服务名保持为 `new-api`，Guard 和 NewAPI 就能在编排内部直接互通。

### 编排内服务说明

- `new-api`
  使用官方镜像启动 NewAPI
- `guard`
  使用镜像直接启动 Guard
- `postgres`
  放在编排最下方，作为 NewAPI 的常见外部数据库依赖示例
- `redis`
  放在编排最下方，作为 NewAPI 的常见外部缓存依赖示例

### 为什么之前会出现 PostgreSQL 和 Redis

那两个服务原本是给 `NewAPI` 准备的，不是给 `Guard` 用的。

现在的处理方式是：

- `Guard` 和 `NewAPI` 都直接用镜像启动
- `Guard` 继续使用自己本地的 SQLite 数据文件
- `PostgreSQL / Redis` 仍然保留在编排最下方，并明确标注为 NewAPI 的常见外部依赖
- 如果你的 NewAPI 当前不需要它们，可以不接入对应环境变量，或自行移除

这样既保留了常见部署参考，也不会让人误以为它们是 Guard 的必需组件。

### 快速启动顺序

推荐按下面的顺序部署：

1. 在根目录准备 `.env`
   可以参考 `.env.example`
2. 先填好至少这几个值：
   - `GUARD_ADMIN_PASSWORD`
   - `GUARD_NEWAPI_URL`
   - `GUARD_IMAGE`
3. 执行启动：

```bash
docker compose up -d
```

4. 打开 NewAPI，完成首次初始化
5. 在 NewAPI 中创建或获取管理员令牌
6. 把这个令牌填回 `.env` 中的 `GUARD_NEWAPI_ADMIN_TOKEN`
7. 重启 Guard：

```bash
docker compose up -d guard
```

### 默认暴露端口

- `NewAPI`：`3000`
- `Guard`：`9000`

可以通过 `.env` 中的下面两个值调整：

- `NEWAPI_PORT`
- `GUARD_PORT`

### 镜像构建方式

Guard 镜像通过 GitHub Action **手动触发构建**，不会在每次提交时自动编译。

工作流文件：

- [`.github/workflows/build-guard-image.yml`](./.github/workflows/build-guard-image.yml)

使用方式：

1. 进入 GitHub 仓库的 `Actions`
2. 选择 `手动构建 Guard 镜像`
3. 手动填写镜像标签，例如 `latest` 或 `v0.1.0`
4. 执行工作流
5. 构建完成后，把生成的镜像地址填到 `.env` 的 `GUARD_IMAGE`

示例：

```text
GUARD_IMAGE=ghcr.io/你的账号/newapi-guard:latest
```

## 管理面板

Guard 管理面板入口：

```text
/guard/admin/
```

如果你本地直接访问 Guard 容器，地址通常是：

```text
http://localhost:9000/guard/admin/
```

登录密码来自：

- 环境变量 `GUARD_ADMIN_PASSWORD`
- 或者 SQLite `config` 表里的 `admin_password`

当前启动逻辑是：

- 如果启动时设置了 `GUARD_ADMIN_PASSWORD`，Guard 会把它写回本地配置表
- 管理面板登录时，会拿输入密码与 `admin_password` 做比对

## Discord OAuth 说明

如果你要启用 Guard 作为 NewAPI 的自定义 OAuth Provider，除了基础环境变量外，还需要在 Guard 后台补齐以下配置：

- `public_base_url`
- `oauth_client_id`
- `oauth_client_secret`
- `discord_client_id`
- `discord_client_secret`
- `discord_guild_id`
- `discord_access_policy`

推荐流程是：

1. 先让 Guard 正常启动并进入后台
2. 在后台把上述配置填完整
3. 再到 NewAPI 管理后台里创建自定义 OAuth Provider
4. 把 `authorize / token / userinfo` 三个地址指向 Guard

### 反向代理与回调地址必读

这一段如果没配严谨，最容易出现下面这类现象：

- 身份组验证成功，但又跳回 NewAPI 登录页
- NewAPI 回调页提示“获取 token 失败”
- 同一个人有时能登录，有时失败

这类问题通常不是项目逻辑本身有错，而是 **反向代理、域名、协议或回调地址不完全一致**。

请务必满足下面这些条件：

1. `public_base_url` 必须固定为用户实际访问的唯一外网地址  
   例如：`https://api.example.com`
2. Discord 开发者后台的 Redirect URL 必须精确配置为  
   `https://api.example.com/guard/oauth/callback/discord`
3. NewAPI 自定义 OAuth Provider 的回调链路实际会使用  
   `https://api.example.com/oauth/guard-discord`
4. 登录时不要混用多个入口  
   例如不要一会儿用域名、一会儿用 IP、一会儿用另一个二级域名
5. 所有涉及 OAuth 的地址必须保持同一协议  
   不要前面用 `https`，后面某一步又变成 `http`

换句话说，下面这几项必须是“同一个外网域名体系”：

- Guard 设置里的 `public_base_url`
- Discord OAuth 回调地址
- 用户实际打开的 NewAPI 登录页域名
- NewAPI 自定义 OAuth Provider 里填写的 Guard 端点

### Nginx / OpenResty 最低要求

如果你前面有 Nginx、OpenResty、1Panel 反代，至少要传这些头：

```nginx
proxy_set_header Host $host;
proxy_set_header X-Forwarded-Proto $scheme;
proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
proxy_set_header X-Real-IP $remote_addr;
```

如果缺少 `Host` 或 `X-Forwarded-Proto`，Guard 在某些场景下可能推断出错误的外部地址，进而导致 OAuth 回调或 token 交换不稳定。

### 推荐的反向代理示例

```nginx
server {
    listen 443 ssl;
    server_name api.example.com;

    location /v1/ {
        proxy_pass http://guard:9000;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_buffering off;
    }

    location = /api/user/checkin {
        proxy_pass http://guard:9000;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location /guard/ {
        proxy_pass http://guard:9000;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location / {
        proxy_pass http://new-api:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

### 现象和排查方向

- Guard 页面显示“身份组验证成功，登录成功”，但 NewAPI 又提示“获取 token 失败”  
  这通常说明 Discord 和 Guard 校验都已经通过，问题多半在 `/guard/oauth/token` 这一步的 `client_id`、`client_secret`、`redirect_uri` 或域名不一致。
- Guard 页面显示“无要求身份组，登录失败”  
  说明已经成功拿到 Discord 用户与服务器成员信息，但没有命中配置的身份组规则。
- 偶发成功、偶发失败  
  优先检查是否混用了多个登录入口，或反代对协议/主机名的透传不稳定。

## 数据目录

当前 Compose 模板会在项目根目录下生成以下数据目录：

- `./data/guard`
- `./data/new-api`
- `./data/postgres`
- `./data/redis`
- `./logs/new-api`

这些目录都已经被 `.gitignore` 忽略，不会被提交到仓库。

## 适合继续完善的方向

如果后面继续打磨，优先建议补这几项：

- 让更多后台配置支持启动时通过环境变量初始化
- 让 `GUARD_TOKEN_CACHE_TTL` 真正作用到代理层 token 缓存
- 增加反向代理示例，例如 Nginx 或 OpenResty
- 增加首启检测，明确提示哪些关键配置还未完成

## 参考

- NewAPI 官方仓库（Docker Compose 与镜像参考）：https://github.com/QuantumNous/new-api

## License

MIT
