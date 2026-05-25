# New API Guard 外挂技术文档

## 一、项目概述

### 1.1 项目定位

Guard 是 New API 的**透明外挂中间件**，为 New API 补充其不具备的功能。Guard 对用户完全透明，用户始终只接触 New API 的界面。Guard 不修改 New API 的任何代码，全程通过 New API 的管理 API 进行交互。

### 1.2 设计原则

- **不侵入**：New API 零修改，保持原版可随时升级
- **透明**：用户不感知 Guard 的存在
- **复用**：用户创建、封禁、额度管理等全部复用 New API 已有接口
- **解耦**：三个功能模块独立，通过共享数据库协作
- **轻量**：Go 单二进制 + SQLite 单文件，无外部依赖

### 1.3 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go |
| 前端 | Vue 3 + Naive UI（CDN 引入，无需构建工具） |
| 数据库 | SQLite（持久化） + Go 内存缓存（热数据） |
| 反向代理 | OpenResty（已有，1Panel 管理） |
| 容器化 | Docker Compose |

---

## 二、系统架构

### 2.1 整体架构

```
互联网用户
    │
    ▼
OpenResty（TLS 终止 + 路由分发）
    │
    ├── /v1/*              → Guard:9000（透明代理模块）
    ├── /api/user/checkin   → Guard:9000（签到拦截）
    ├── /guard/*            → Guard:9000（管理面板 + Guard OAuth Provider + 可选静态脚本）
    └── /*（其余）          → New-API:3000（原生前端 + API）
    
    
Guard:9000                          New-API:3000
┌──────────────────────┐           ┌──────────────────┐
│  模块A: OAuth Provider│──OAuth联动──→│                  │
│  模块B: 管理面板     │──管理API──→│   New API 原版    │
│  模块C: 透明代理     │──转发请求──→│   不做任何修改    │
│         │            │           │                  │
│     SQLite.db        │           │  自带用户系统     │
│   (共享数据库)       │           │  自带Token管理    │
└──────────────────────┘           │  自带额度系统     │
                                   └──────────────────┘
```

### 2.2 容器网络

Guard 和 New API 在同一 Docker Compose 编排中，通过 Docker 内部网络以 service name 互通：

```
Guard 访问 New API：http://new-api:3000
OpenResty 访问 Guard：http://guard:9000
OpenResty 访问 New API：http://new-api:3000
```

Guard 转发用户请求到 New API 时，目标地址为 `http://new-api:3000`，不经过域名和外部网络。

### 2.3 OpenResty 路由配置

```nginx
server {
    listen 443 ssl;
    server_name 你的newapi域名;

    # 可选：仅当需要微调前端文案时，才注入低优先级脚本
    # sub_filter '</head>' '<script src="/guard/static/inject.js"></script></head>';
    # sub_filter_once on;
    # sub_filter_types text/html;

    # API 请求 → Guard 透明代理
    location /v1/ {
        proxy_pass http://guard:9000;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_buffering off;  # 不缓冲，支持 SSE 流式透传
    }

    # 签到拦截 → Guard
    location = /api/user/checkin {
        proxy_pass http://guard:9000;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }

    # Guard 管理面板 + OAuth Provider + 可选静态资源
    location /guard/ {
        proxy_pass http://guard:9000;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # 其余所有 → New API 原版
    location / {
        proxy_pass http://new-api:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

**关键点：**

- `proxy_buffering off` 确保 `/v1/chat/completions` 等流式 SSE 响应直接透传到用户，Guard 不缓冲不处理流内容。
- `Host` 与 `X-Forwarded-Proto` 必须透传，否则 Guard 在 OAuth 流程里推断外部地址时可能得到错误的协议或主机名。
- 对外只应保留一个正式访问域名，不要混用域名、IP、备用二级域名，否则 OAuth 的 `redirect_uri` 很容易出现偶发不一致。

---

## 三、数据库设计

### 3.1 SQLite 表结构

所有持久化数据存储在单个 SQLite 文件中，三个模块共享访问。

**用户绑定表 `users`**

存储 New API 用户与 Discord 账号的关联关系。

| 字段 | 类型 | 说明 |
|------|------|------|
| newapi_user_id | INTEGER PK | New API 的用户 ID |
| discord_id | TEXT, UNIQUE, 可空 | Discord 用户 ID（手动创建的账号为空） |
| discord_name | TEXT, 可空 | Discord 用户名（便于辨识） |
| is_whitelist | BOOLEAN, 默认 false | 是否白名单用户 |
| created_at | DATETIME | 创建时间 |

**Token 缓存表 `token_cache`**

缓存 API Key 与用户的对应关系，避免每次都调 New API 查询。

| 字段 | 类型 | 说明 |
|------|------|------|
| token_key | TEXT PK | API Key（sk-xxx） |
| newapi_user_id | INTEGER FK → users | 所属用户 |
| cached_at | DATETIME | 缓存时间 |

**UA 违规记录表 `ua_strikes`**

| 字段 | 类型 | 说明 |
|------|------|------|
| newapi_user_id | INTEGER PK | 用户 ID |
| count | INTEGER, 默认 0 | 累计违规次数 |
| last_ua | TEXT | 最近一次违规的 UA |
| last_strike_at | DATETIME | 最近违规时间 |

**封禁记录表 `bans`**

Guard 仅记录封禁的上下文信息，实际封禁状态以 New API 的 `users.status` 字段为准（status=2 为封禁）。Guard 不维护独立的封禁状态。

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增 |
| newapi_user_id | INTEGER | 用户 ID |
| discord_id | TEXT, 可空 | DC ID（溯源用） |
| reason | TEXT | 封禁原因 |
| violation_ua | TEXT, 可空 | 触发封禁的 UA |
| client_ip | TEXT, 可空 | 来源 IP |
| duration | TEXT | "permanent" / "7d" / "30d" |
| expire_at | DATETIME, 可空 | 临时封禁到期时间 |
| unbanned_at | DATETIME, 可空 | 解封时间（为空表示仍在封禁） |
| created_at | DATETIME | 封禁时间 |

**签到记录表 `checkin_records`**

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增 |
| newapi_user_id | INTEGER | 用户 ID |
| quota_added | INTEGER | 本次签到获得的额度 |
| checked_at | DATE | 签到日期 |
| UNIQUE(newapi_user_id, checked_at) | | 每用户每天最多一条 |

**OAuth 授权码表 `oauth_authorization_codes`**

用于 Guard 作为 OAuth Provider 时，缓存短时有效的授权码。

| 字段 | 类型 | 说明 |
|------|------|------|
| code | TEXT PK | Guard 生成的一次性 authorization code |
| discord_id | TEXT | Discord 用户 ID |
| discord_name | TEXT, 可空 | Discord 用户名/显示名 |
| payload | TEXT | 序列化后的用户信息快照 |
| expire_at | DATETIME | 过期时间（建议 60-120 秒） |
| used_at | DATETIME, 可空 | 被 token endpoint 消费的时间 |
| created_at | DATETIME | 创建时间 |

**系统配置表 `config`**

所有可通过管理面板修改的配置项。

| 字段 | 类型 | 说明 |
|------|------|------|
| key | TEXT PK | 配置键名 |
| value | TEXT | 配置值（JSON 格式） |

预置配置项：

| key | 默认 value | 说明 |
|-----|-----------|------|
| `rpm_limit` | `3` | 全局默认 RPM |
| `ua_ban_strikes` | `3` | UA 违规封号次数 |
| `allowed_ua` | `[]` | 允许的 UA 前缀列表（留空表示不启用） |
| `checkin_quota` | `500000` | 每次签到获得的额度 |
| `checkin_threshold` | `200000` | 余额低于此值才能签到 |
| `oauth_client_id` | `""` | Guard 作为 OAuth Provider 暴露给 New API 的 client_id |
| `oauth_client_secret` | `""` | Guard 作为 OAuth Provider 暴露给 New API 的 client_secret |
| `oauth_provider_slug` | `"guard-discord"` | 在 New API 中创建的自定义 OAuth Provider slug |
| `discord_client_id` | `""` | Guard 连接 Discord OAuth 的客户端 ID |
| `discord_client_secret` | `""` | Guard 连接 Discord OAuth 的客户端密钥 |
| `discord_guild_id` | `""` | 要求加入的 Discord 服务器 ID |
| `discord_access_policy` | `{"logic":"and","conditions":[],"groups":[]}` | Discord 准入规则，支持 AND/OR 组合 |
| `admin_password` | `""` | 管理面板登录密码 |
| `newapi_admin_token` | `""` | New API 管理员 Token |

**每日统计表 `daily_stats`**

| 字段 | 类型 | 说明 |
|------|------|------|
| date | TEXT PK | 日期 "2026-05-24" |
| total_requests | INTEGER, 默认 0 | 当日 /v1/ 总请求数 |
| blocked_ua | INTEGER, 默认 0 | UA 拦截次数 |
| blocked_rpm | INTEGER, 默认 0 | RPM 拦截次数 |
| checkins | INTEGER, 默认 0 | 签到次数 |
| new_users | INTEGER, 默认 0 | 新注册用户数 |
| new_bans | INTEGER, 默认 0 | 新封禁数 |

### 3.2 内存缓存

以下数据在 Go 进程内存中缓存，加速请求处理：

| 缓存项 | 数据结构 | 来源 | 失效策略 |
|--------|---------|------|---------|
| token → user_id | `sync.Map` | 首次从 New API 查询后写入，同时写 SQLite | TTL 5 分钟自动过期，过期后重新查询 |
| user_id → is_whitelist | `sync.Map` | 从 SQLite 加载 | 管理面板修改白名单时主动刷新 |
| 系统配置 | 结构体 | 从 SQLite 加载 | 管理面板修改时主动刷新 |
| RPM 计数 | `sync.Map` key=`{user_id}:{minute}` | 运行时累加 | 每分钟自动清理过期条目 |
| 当日统计 | 结构体 + atomic 计数 | 从 SQLite 加载 | 事件发生时内存累加，定期（如每 30 秒）批量写回 SQLite |

**RPM 计数不持久化**：Guard 重启后计数归零，最坏情况某用户在重启的那一分钟内多了几个请求，可接受。

**UA 违规计数持久化到 SQLite**：防止用户通过等待 Guard 重启来清零违规次数。

---

## 四、模块A：Discord 准入登录（Guard 作为 New API 自定义 OAuth Provider）

### 4.1 功能说明

当前版本的 New API 已经内置两类与 OAuth 相关的能力：

1. 内置 Discord OAuth 开关：可在 New API 管理面板中直接开启 Discord 登录按钮
2. 自定义 OAuth Provider：可在 New API 管理面板中配置一个外部 OAuth 提供方，登录页自动显示按钮

由于内置 `discord_oauth` 只负责普通 Discord 登录，不支持“必须在指定服务器、必须满足指定身份组组合规则”的准入要求，因此 Guard 的推荐实现方式不是接管 New API 登录页 DOM，而是：

- **New API 负责显示登录按钮与最终建立登录会话**
- **Guard 负责充当自定义 OAuth Provider，并在内部完成 Discord 鉴权与准入规则校验**

这样按钮显示、OAuth 回调、用户创建、登录 session 建立都由 New API 原生机制完成；Guard 只补足 Discord 准入控制。

### 4.2 与 New API 的联动方式

**New API 侧使用的真实接口：**

```
GET    /api/status
  → 返回 custom_oauth_providers
  → 前端据此自动渲染“继续使用 xxx 登录”按钮

GET    /api/oauth/state
  → 生成 OAuth state，用于前端发起登录

GET    /api/oauth/{provider}
  → New API 后端处理 OAuth 回调后的 code 换 token、取 userinfo、创建/查找用户、建立 session

GET    /api/custom-oauth-provider/
POST   /api/custom-oauth-provider/
GET    /api/custom-oauth-provider/{id}
PUT    /api/custom-oauth-provider/{id}
DELETE /api/custom-oauth-provider/{id}
  → New API 超级管理员在面板中配置自定义 OAuth Provider
```

**Guard 侧需要暴露标准 OAuth Provider 接口：**

```
GET  /guard/oauth/authorize
POST /guard/oauth/token
GET  /guard/oauth/userinfo
```

**New API 中创建的自定义 OAuth Provider 推荐配置：**

| 字段 | 建议值 |
|------|--------|
| name | `Discord` 或 `Guard Discord` |
| slug | `guard-discord` |
| enabled | `true` |
| client_id | `config.oauth_client_id` |
| client_secret | `config.oauth_client_secret` |
| authorization_endpoint | `https://你的newapi域名/guard/oauth/authorize` |
| token_endpoint | `https://你的newapi域名/guard/oauth/token` |
| user_info_endpoint | `https://你的newapi域名/guard/oauth/userinfo` |
| scopes | `openid profile email` |
| user_id_field | `sub` |
| username_field | `preferred_username` |
| display_name_field | `name` |
| email_field | `email` |

**重要说明：**

- 推荐使用 **New API 的 custom oauth provider** 接入 Guard，不依赖 New API 内置 `discord_oauth`
- New API 管理面板中可开启内置 `discord_oauth`，但它不参与 Guard 的准入校验主流程
- 自定义 Provider 配置完成后，New API 登录页会自动显示按钮，**默认不需要 JS 注入**
- `client_id`、`client_secret`、`authorization_endpoint`、`token_endpoint`、`user_info_endpoint` 建议全部填写为同一正式 HTTPS 域名下的地址，不要混用 IP 或内网地址

### 4.3 登录与跳转流程

**完整流程：**

```
步骤1：管理员在 New API 面板中创建自定义 OAuth Provider
  → 使用 /api/custom-oauth-provider/* 配置 Guard 的 authorize/token/userinfo endpoint
  → New API 的 /api/status 返回 custom_oauth_providers

步骤2：用户打开 New API 登录页
  → New API 前端读取 /api/status
  → 自动渲染 “Continue with Discord / 使用 Discord 登录” 按钮

步骤3：用户点击按钮
  → New API 前端先调用 GET /api/oauth/state
  → 然后跳转到：
     GET /guard/oauth/authorize
       ?client_id=...
       &redirect_uri=https://你的newapi域名/oauth/guard-discord
       &response_type=code
       &scope=openid%20profile%20email
       &state=...

步骤4：Guard 作为 OAuth Provider 处理 authorize 请求
  → 校验 client_id、redirect_uri、response_type、state
  → 若用户尚未完成 Discord 鉴权，Guard 重定向到 Discord：
     https://discord.com/oauth2/authorize
     scope: identify guilds.members.read
  → Discord 回调到 Guard 的内部处理地址

步骤5：Guard 获取 Discord 用户资料并校验准入规则
  → POST https://discord.com/api/v10/oauth2/token
  → GET  https://discord.com/api/v10/users/@me
  → GET  https://discord.com/api/users/@me/guilds/{guild_id}/member
  → 使用 config.discord_access_policy 判定是否允许登录
  → 失败：显示拒绝页或重定向回 New API 回调页并带错误

步骤6：Guard 签发自己的 authorization code
  → 写入 oauth_authorization_codes 表
  → 302 跳转回：
     https://你的newapi域名/oauth/guard-discord?code=xxx&state=yyy

步骤7：New API 前端进入 /oauth/{provider} 回调页
  → 前端调用：
     GET /api/oauth/guard-discord?code=xxx&state=yyy

步骤8：New API 后端用 code 调 Guard 的 token endpoint
  → POST /guard/oauth/token
     grant_type=authorization_code
     code=xxx
     redirect_uri=https://你的newapi域名/oauth/guard-discord
     client_id=config.oauth_client_id
     client_secret=config.oauth_client_secret

步骤9：New API 后端再调 Guard 的 userinfo endpoint
  → GET /guard/oauth/userinfo
     Authorization: Bearer {guard_access_token}
  → Guard 返回标准用户信息：
     {
       "sub": "discord:123456789",
       "preferred_username": "dc_123456789",
       "name": "Tom",
       "email": ""
     }

步骤10：New API 完成用户创建/查找并建立登录态
  → 由 New API 自己调用内部 OAuth 创建/绑定逻辑
  → 新用户是否允许创建，取决于 New API 自身的注册开关
  → 登录成功后，New API 调用 setupLogin 写入 session
  → 前端随后调用 /api/user/self，进入已登录状态
```

### 4.3.1 部署时必须满足的“完全一致”

下面几项如果不完全一致，就可能出现“身份组验证成功，但回到 NewAPI 登录页”或“获取 token 失败”：

1. Guard 设置中的 `public_base_url`
2. 用户实际访问 NewAPI 登录页时看到的外网域名
3. Discord 开发者后台登记的 Redirect URL  
   `https://你的newapi域名/guard/oauth/callback/discord`
4. NewAPI 在 OAuth 流程中使用的回调地址  
   `https://你的newapi域名/oauth/guard-discord`
5. NewAPI 自定义 OAuth Provider 中配置的 Guard 三个端点

注意事项：

- 协议必须一致：前后都应是 `https`
- 主机名必须一致：不要一部分请求走域名，另一部分走 IP
- 路径必须一致：不要一个地址带尾部斜杠、另一个不带
- 同一用户登录时不要反复点击按钮，否则旧 `code` 可能被覆盖或提前失效

这类问题往往不是 Discord 身份组判断错误，而是 OAuth 后半段的 `redirect_uri`、`code` 或客户端凭据校验失败。

### 4.4 会话机制说明

开发时需明确当前版本 New API 的真实登录机制：

- 最终登录态由 **New API 自己建立**
- New API 登录成功后会写入服务端 session，并在前端保存 `uid/user` 到 `localStorage`
- 后续浏览器请求依赖：
  - Cookie session
  - `New-Api-User` 请求头

因此 Guard **不需要也不应该** 自己拼装 `access_token` 再跳回前端；只需作为 OAuth Provider 返回标准 `code/token/userinfo`，由 New API 原生流程完成最终登录。

### 4.5 Discord 准入规则

Guard 的准入规则由 `config.discord_access_policy` 控制，建议使用可嵌套的 JSON 结构，而不是简单的“单一角色列表”。

示例：

```json
{
  "logic": "and",
  "conditions": [
    { "field": "guild_id", "op": "eq", "value": "123456789012345678" }
  ],
  "groups": [
    {
      "logic": "or",
      "conditions": [
        { "field": "roles", "op": "contains", "value": "role_a" },
        { "field": "roles", "op": "contains", "value": "role_b" }
      ]
    },
    {
      "logic": "and",
      "conditions": [
        { "field": "roles", "op": "contains", "value": "role_c" },
        { "field": "roles", "op": "contains", "value": "role_d" }
      ]
    }
  ]
}
```

这样可以表达：

- 必须在指定服务器中
- 满足 A 或 B 任一身份组
- 或者同时满足 C 和 D 两个身份组

规则中应优先保存 `guild_id` 与 `role_id`，名称仅用于管理面板展示，避免 Discord 改名导致规则失效。

---

## 五、模块B：管理运维面板

### 5.1 功能说明

为管理员提供一个 Web 管理面板，统一管理用户、白名单、封禁、系统设置和运维统计。面板入口为 `https://你的newapi域名/guard/admin/`，仅管理员使用。

### 5.2 鉴权

管理面板使用静态密码鉴权：

```
POST /guard/api/auth/login    { "password": "xxx" }
  → 密码与 config.admin_password 比对
  → 通过：返回一个 session token（JWT 或随机串），设置有效期
  → 失败：返回 401

后续所有管理 API 请求：
  Header: Authorization: Bearer {session_token}
  → Guard 校验 token 有效性
  → 无效/过期：返回 401
```

### 5.3 管理 API 接口设计

所有管理 API 前缀为 `/guard/api/`，需携带有效的管理员 session token。

**5.3.1 总览**

```
GET /guard/api/dashboard
  → 返回：
    {
      "today": {                          // 来自 SQLite daily_stats
        "total_requests": 1234,
        "blocked_ua": 5,
        "blocked_rpm": 23,
        "checkins": 15,
        "new_users": 3,
        "new_bans": 1
      },
      "total_users": 89,                 // 来自 SQLite users 表 COUNT
      "active_bans": 3,                  // 来自 New API 查询 status=2 的用户数
      "whitelist_count": 5               // 来自 SQLite users 表 WHERE is_whitelist=1
    }
```

**5.3.2 用户管理**

```
GET /guard/api/users?page=1&size=20&search=xxx
  → 来源：SQLite users 表
  → 对每个用户，调 New API 管理 API 获取实时信息（余额、状态）
  → 返回合并后的列表：
    [{
      "newapi_user_id": 42,
      "username": "dc_298374...",
      "discord_id": "298374928374",
      "discord_name": "Tom#1234",
      "is_whitelist": false,
      "status": 1,                       // 来自 New API（1=正常 2=封禁）
      "quota": 350000,                   // 来自 New API
      "created_at": "2026-05-20T..."
    }, ...]

POST /guard/api/users
  → 手动创建用户
  → 请求体两种模式：

  账号密码模式：
    {
      "mode": "password",
      "username": "tom",
      "password": "123456",
      "initial_quota": 500000,           // 可选，默认 0
      "is_whitelist": false              // 可选，默认 false
    }
    → Guard 调 New API: POST /api/user/ 创建用户
    → 拿到 user_id
    → 写入 SQLite users 表（discord_id 为空）
    → 如果有初始额度，调 New API 充值 API

  DC 绑定模式：
    {
      "mode": "discord",
      "discord_id": "298374928374",
      "discord_name": "Tom#1234",
      "initial_quota": 500000,
      "is_whitelist": false
    }
    → Guard 调 New API: POST /api/user/ 创建用户（username: dc_{discord_id}）
    → 写入 SQLite users 表（含 discord_id）
    → 此模式跳过身份组校验，管理员特权

GET /guard/api/users/{newapi_user_id}
  → 单个用户详情
  → 合并 SQLite 数据 + New API 实时数据
  → 包含：绑定信息、封禁历史、签到记录、UA 违规次数
```

**5.3.3 白名单管理**

```
GET /guard/api/whitelist
  → 来源：SQLite users 表 WHERE is_whitelist = 1
  → 返回白名单用户列表

POST /guard/api/whitelist/{newapi_user_id}
  → 将用户加入白名单
  → 更新 SQLite: users.is_whitelist = true
  → 刷新内存缓存

DELETE /guard/api/whitelist/{newapi_user_id}
  → 将用户移出白名单
  → 更新 SQLite: users.is_whitelist = false
  → 刷新内存缓存

白名单用户豁免范围：
  ✅ 不受 RPM 限制
  ✅ 不受 UA 检查
  ✅ 不会被自动封号
  ❌ 额度仍正常扣减（白名单 ≠ 无限额度）
```

**5.3.4 封禁管理**

```
GET /guard/api/bans?status=active|all
  → 获取封禁记录列表
  → status=active：仅当前生效的封禁
     → 来源：查 New API 获取 status=2 的用户列表
     → 匹配 SQLite bans 表补充上下文（原因、时间等）
     → 某些用户在 New API 中 status=2 但 SQLite 无记录：标注"无上下文（可能从 New API 后台直接封禁）"
  → status=all：全部历史记录，来自 SQLite bans 表

POST /guard/api/bans
  → 手动封禁用户
  → 请求体：
    {
      "newapi_user_id": 42,
      "reason": "违规使用",
      "duration": "permanent"            // "permanent" / "7d" / "30d"
    }
  → Guard 处理：
    ① 调 New API: PUT /api/user/ { "id": 42, "status": 2 }（执行封禁）
    ② 查 SQLite users 表获取 discord_id（溯源）
    ③ 写入 SQLite bans 表（记录上下文）
    ④ 如果 duration 非 permanent，计算 expire_at
    ⑤ 清除该用户的内存缓存
    ⑥ daily_stats.new_bans += 1

POST /guard/api/bans/{ban_id}/unban
  → 解封用户
  → Guard 处理：
    ① 从 SQLite bans 表查到 newapi_user_id
    ② 调 New API: PUT /api/user/ { "id": xx, "status": 1 }（解除封禁）
    ③ 更新 SQLite bans 表: unbanned_at = 当前时间
    ④ 清除该用户的 ua_strikes 记录（给予重新机会）
    ⑤ 刷新内存缓存
```

**临时封禁自动解封：**

Guard 启动一个定时任务（每分钟执行一次），扫描 SQLite bans 表中 `expire_at` 非空且已到期、`unbanned_at` 为空的记录，逐一执行解封流程。

**5.3.5 系统设置**

```
GET /guard/api/settings
  → 返回所有配置项（来自 SQLite config 表）

PUT /guard/api/settings
  → 批量更新配置
  → 请求体：
    {
      "rpm_limit": 3,
      "ua_ban_strikes": 3,
      "allowed_ua": [],
      "checkin_quota": 500000,
      "checkin_threshold": 200000,
      ...
    }
  → 写入 SQLite config 表
  → 刷新内存中的配置缓存（立即生效）
```

**5.3.6 日志查询**

```
GET /guard/api/logs/bans?page=1&size=50
  → 封禁日志（SQLite bans 表）

GET /guard/api/logs/checkins?page=1&size=50&user_id=42
  → 签到日志（SQLite checkin_records 表）

GET /guard/api/logs/stats?days=30
  → 每日统计趋势（SQLite daily_stats 表，最近 N 天）
```

**5.3.7 与 New API 的 OAuth 联动配置**

Guard 的 Discord 准入规则在 Guard 面板维护，但登录按钮的显示与 Provider 挂载由 New API 负责。

推荐操作顺序：

1. 在 Guard 面板中配置：
   - `oauth_client_id`
   - `oauth_client_secret`
   - `discord_client_id`
   - `discord_client_secret`
   - `discord_guild_id`
   - `discord_access_policy`
2. 在 New API 超级管理员面板中创建一个 custom OAuth provider：
   - `authorization_endpoint=/guard/oauth/authorize`
   - `token_endpoint=/guard/oauth/token`
   - `user_info_endpoint=/guard/oauth/userinfo`
   - `client_id/client_secret` 填 Guard 生成或配置的 OAuth client 凭据
3. 启用该 provider 后，New API 登录页自动显示按钮

这样可以实现：

- **New API 控制按钮显示与最终登录 session**
- **Guard 控制 Discord 准入规则**
- **两者通过标准 OAuth provider 配置互相关联**

### 5.4 前端页面

前端为纯静态文件，由 Guard 伺服，存放于项目 `web/` 目录。使用 Vue 3 和 Naive UI 通过 CDN 引入，不需要 Node.js 或构建工具。

```
GET /guard/admin/           → web/index.html（管理面板 SPA）
GET /guard/static/inject.js → web/inject.js（注入到 New API 的脚本）
GET /guard/static/*         → web/ 下的静态资源
```

**页面结构：**

```
guard/admin/
├── 登录页        输入静态密码
├── 总览          当日统计 + 总量统计
├── 用户管理      用户列表（搜索/创建/查看详情）
├── 白名单        白名单用户列表（添加/移除）
├── 封禁管理      封禁列表（封禁/解封/查看历史）
├── 系统设置      RPM / UA列表 / 签到参数 / Discord 准入规则 / Guard OAuth 凭据
└── 日志          封禁日志 / 签到日志 / 每日统计图表
```

---

## 六、模块C：透明代理

### 6.1 功能说明

对所有经过 `/v1/*` 的 API 请求进行透明代理，在转发到 New API 之前执行 UA 白名单检查和用户级 RPM 限速。对用户完全透明——通过的请求原样转发，被拦截的请求返回标准错误 JSON。

### 6.2 请求处理流程

```
请求进入 Guard /v1/*
    │
    ├─ HTTP 方法是 GET 或 OPTIONS？
    │   └─ 是 → 直接透传到 New API，不做任何检查，流程结束
    │
    └─ 是 POST → 进入检查流程
         │
         │  ① 提取 Token
         │  依次检查以下位置，取到即停：
         │    a. Authorization 头 → 去掉 "Bearer " 前缀
         │    b. x-api-key 头
         │    c. api-key 头
         │  全部没有 → 不拦截，透传到 New API（让 New API 自己返回 401）
         │
         │  ② Token → User ID 解析（带缓存）
         │  查内存缓存 → 命中？取 user_id
         │                未命中？→ 查 SQLite token_cache 表
         │                          命中？取 user_id，写入内存缓存
         │                          未命中？→ 调 New API 管理 API 查询
         │                                    GET http://new-api:3000/api/token/search?keyword={token}
         │                                    Authorization: Bearer {config.newapi_admin_token}
         │                                    → 拿到 user_id
         │                                    → 写入 SQLite token_cache 表
         │                                    → 写入内存缓存
         │                                    → 查 SQLite users 表是否已有该 user_id
         │                                      无记录 → 插入 users 表（discord_id 为空）
         │                                    查询失败（token 无效）→ 不拦截，透传让 New API 拒绝
         │
         │  ③ 白名单检查
         │  查内存缓存 user_id 是否在白名单中
         │  是白名单 → 跳过所有后续检查，直接透传到 New API
         │
         │  ④ UA 检查
         │  读取 User-Agent 请求头
         │  与 config.allowed_ua 列表逐一前缀匹配
         │  匹配到任一 → 通过
         │  全不匹配 → UA 违规处理：
         │    → SQLite ua_strikes 表该用户 count += 1
         │    → count < config.ua_ban_strikes：
         │        返回 HTTP 403
         │        { "error": { "message": "Unauthorized client ({count}/{max})", "type": "access_denied" } }
         │    → count >= config.ua_ban_strikes：
         │        触发自动封禁（流程同管理模块的封禁操作）：
         │          调 New API 封禁用户
         │          写 SQLite bans 表（reason: "ua_violation", 含 UA 和 IP）
         │          查 users 表获取 discord_id 用于溯源记录
         │        返回 HTTP 403
         │        { "error": { "message": "Account banned: unauthorized client", "type": "access_denied" } }
         │    → daily_stats.blocked_ua += 1
         │
         │  ⑤ RPM 检查（用户级别）
         │  RPM key = {user_id}:{当前分钟时间戳}
         │  在内存 map 中递增计数
         │  count <= config.rpm_limit → 通过
         │  count > config.rpm_limit →
         │    返回 HTTP 429
         │    { "error": { "message": "Rate limit: {rpm_limit} requests/min", "type": "rate_limit_error" } }
         │    daily_stats.blocked_rpm += 1
         │
         │  ⑥ 全部通过
         │  daily_stats.total_requests += 1
         │  将请求原样转发到 http://new-api:3000/v1/{path}
         │  透传所有请求头（移除 Hop-by-hop 头）
         │  响应不缓冲，直接流式透传回用户（支持 SSE）
```

### 6.3 Token 提取兼容性

不同 API 格式的客户端使用不同的请求头传递 Key：

| 格式 | 请求头 | 示例 |
|------|--------|------|
| OpenAI | `Authorization` | `Bearer sk-xxx` |
| Anthropic / Claude | `x-api-key` | `sk-xxx` |
| Azure OpenAI | `api-key` | `sk-xxx` |

Guard 按上述优先级依次尝试提取，提取后统一为 `sk-xxx` 进入后续流程。

### 6.4 签到拦截

对 `POST /api/user/checkin` 的请求，Guard 拦截后**不转发到 New API**，而是自行处理签到逻辑。

```
请求进入 Guard /api/user/checkin
    │
    │  ① 识别用户身份
    │  请求来自 New API 前端（浏览器），携带：
    │    - New API 的 session cookie
    │    - New-Api-User 请求头
    │  Guard 将这些信息原样透传给 New API 的用户信息接口：
    │    GET http://new-api:3000/api/user/self
    │    Cookie: {原样透传}
    │    New-Api-User: {原样透传}
    │    Authorization: {若原请求存在则一并透传}
    │  New API 返回当前用户信息（user_id, quota 等）
    │  失败 → 返回 401
    │
    │  ② 检查是否今日已签到
    │  查 SQLite checkin_records 表
    │  WHERE newapi_user_id = {user_id} AND checked_at = {今天日期}
    │  已有记录 → 返回错误: "今日已签到"
    │
    │  ③ 检查余额
    │  用步骤①返回的 quota 与 config.checkin_threshold 比较
    │  quota >= threshold → 返回错误: "余额充足，无需签到"
    │
    │  ④ 执行签到
    │  Guard 自行完成额度发放（实现方式二选一）：
    │
    │  方案A：继续使用 Guard 自己的签到策略（推荐）
    │    → 用管理员能力为用户增加固定额度
    │    → 写入 SQLite checkin_records 表
    │
    │  方案B：直接复用 New API 原生签到（兼容模式）
    │    → 直接透传到 New API 原生签到实现
    │    → 不做固定额度与余额阈值改写
    │
    │  若采用方案A，需调用 New API 可用的管理员额度调整接口
    │  （实现时以当前版本 New API 管理接口为准）
    │
    │  示例：
    │    POST http://new-api:3000/api/user/topup/complete
    │    Authorization: Bearer {config.newapi_admin_token}
    │    Body: { "user_id": {user_id}, "quota": config.checkin_quota, ... }
    │  写入 SQLite checkin_records 表
    │  daily_stats.checkins += 1
    │
    │  ⑤ 返回成功
    │  返回 JSON（模仿 New API 签到成功的响应格式，使前端正常显示）
    │  {
    │    "success": true,
    │    "message": "签到成功",
    │    "data": {
    │      "quota_awarded": config.checkin_quota,
    │      "checkin_date": "2026-05-24"
    │    }
    │  }
```

**重要说明：**

- 当前版本 New API 前端会先调用 `GET /api/user/checkin?month=YYYY-MM` 拉取签到状态，再调用 `POST /api/user/checkin`
- 当前版本 New API 原生签到成功结构为：

```json
{
  "success": true,
  "message": "签到成功",
  "data": {
    "quota_awarded": 500000,
    "checkin_date": "2026-05-24"
  }
}
```

- Guard 若接管签到，应尽量保持该结构一致
- “将签到页提示文案改成模糊描述”是**超低优先级 UI 优化**，不影响核心开发

### 6.5 转发行为细节

Guard 作为透明代理，转发请求时需注意：

- **请求转发**：原样透传所有请求头（Host 头替换为目标地址），原样透传请求 Body
- **响应转发**：不缓冲响应 Body，直接流式透传；原样返回状态码和响应头
- **SSE 支持**：当 New API 返回 `Content-Type: text/event-stream` 时，Guard 必须逐块转发，不能等待响应结束
- **超时设置**：转发超时建议 300 秒（长对话场景），空闲超时建议 120 秒

---

## 七、前端注入脚本 inject.js（可选，超低优先级）

### 7.1 注入方式

默认情况下，**Discord 登录按钮不需要注入脚本**。当前版本 New API 会根据 `/api/status` 返回的 `custom_oauth_providers` 自动渲染登录按钮。

`inject.js` 仅作为可选的 UI 微调脚本存在。若未来需要调整前端文案，可通过 OpenResty 的 `sub_filter` 在 New API 返回的 HTML 中插入脚本引用。脚本由 Guard 伺服。

### 7.2 脚本职责

`inject.js` 是一个纯前端脚本，**不调用任何 Guard API**，仅做 DOM 修改。

当前仅保留一个低优先级职责：

- 检测到签到相关组件已渲染
- 将 New API 原生的签到描述文字替换为："余额过高时无法签到，每次获得固定额度"
- 不修改按钮行为——按钮仍调用 New API 原生签到路径（`POST /api/user/checkin`），该路径已被 OpenResty 转发到 Guard 处理

### 7.3 注意事项

- `inject.js` 不属于登录主流程，不应阻塞开发和上线
- New API 是 SPA，页面内容动态渲染，`inject.js` 必须等待目标元素出现再修改
- 选择器应尽量使用稳定的特征（如按钮文本内容、表单结构），而非 class 名（class 名可能随版本变化）
- New API 版本更新后需验证 `inject.js` 是否仍然正常工作

---

## 八、部署方案

### 8.1 Docker Compose

```yaml
services:
  guard:
    build: ./guard
    container_name: guard
    restart: always
    volumes:
      - ./guard/data:/app/data          # SQLite 文件持久化
      - ./guard/web:/app/web            # 前端静态文件
    environment:
      - NEWAPI_URL=http://new-api:3000  # 容器内部通信
      - LISTEN_ADDR=:9000
    mem_limit: 64m
    networks:
      - stack-net

  new-api:
    image: calciumion/new-api:latest
    container_name: new-api
    restart: always
    volumes:
      - ./new-api/data:/data
    mem_limit: 512m
    networks:
      - stack-net

networks:
  stack-net:
    driver: bridge
```

### 8.2 Guard 项目目录结构

```
guard/
├── main.go                    入口
├── go.mod
├── go.sum
├── Dockerfile
├── internal/
│   ├── config/
│   │   └── config.go          配置加载（SQLite config 表 → 内存结构体）
│   ├── database/
│   │   └── sqlite.go          SQLite 初始化、建表、通用操作
│   ├── cache/
│   │   └── cache.go           内存缓存（token映射、白名单、RPM计数、统计计数）
│   ├── newapi/
│   │   └── client.go          New API 管理 API 调用封装
│   │                          （创建用户、封禁/解封、充值、查询Token、查询用户等）
│   ├── discord/
│   │   ├── oauth.go           Guard 作为 OAuth Provider 的 Discord 上游鉴权
│   │   └── handler.go         /guard/oauth/* 路由处理
│   ├── proxy/
│   │   ├── middleware.go      UA检查 + RPM限速 + Token解析 逻辑
│   │   ├── handler.go         /v1/* 反向代理转发
│   │   └── checkin.go         /api/user/checkin 拦截处理
│   ├── admin/
│   │   ├── auth.go            管理面板鉴权（静态密码 + session）
│   │   ├── handler.go         /guard/api/* 管理 API 路由
│   │   ├── users.go           用户管理接口
│   │   ├── whitelist.go       白名单接口
│   │   ├── bans.go            封禁管理接口
│   │   ├── settings.go        系统设置接口
│   │   ├── dashboard.go       总览接口
│   │   └── logs.go            日志查询接口
│   └── tasks/
│       └── scheduler.go       定时任务（临时封禁自动解封、RPM计数清理、统计写回）
├── web/                       前端静态文件
│   ├── index.html             管理面板 SPA（Vue 3 + Naive UI CDN）
│   ├── inject.js              可选前端文案微调脚本（非登录主流程）
│   └── assets/                图标等静态资源
└── data/                      运行时数据（Docker volume 挂载）
    └── guard.db               SQLite 数据库文件
```

### 8.3 模块间依赖关系

```
internal/config      ← 被所有模块依赖
internal/database    ← 被所有模块依赖
internal/cache       ← 被所有模块依赖
internal/newapi      ← 被 discord、proxy、admin 依赖

internal/discord     → 独立模块，写 database，调 newapi
internal/proxy       → 独立模块，读 cache/database，调 newapi
internal/admin       → 独立模块，读写 database/cache，调 newapi
internal/tasks       → 独立模块，读写 database，调 newapi

三个功能模块不互相依赖，通过 database 和 cache 共享数据
```

### 8.4 资源预估

| 组件 | 内存占用 |
|------|---------|
| Guard（Go 二进制） | ~20-40MB |
| SQLite | ~1-5MB（取决于数据量） |
| New API | ~200-400MB |
| OpenResty | 已有，不额外占用 |
| **总计新增** | **~30-50MB** |

---

## 九、后续扩展点

以下功能在当前方案中预留了扩展空间，但不在首期实现范围内：

| 扩展项 | 扩展方式 |
|--------|---------|
| Gemini API 格式支持 | OpenResty 加一条 `/v1beta/` 路由；proxy 模块的 Token 提取逻辑增加 URL 参数 `?key=` 来源 |
| 模型分组 RPM | proxy 模块读取 POST Body 中的 `model` 字段，匹配 config 中的模型分组规则，应用不同 RPM |
| 用户组 RPM | 结合 New API 的用户分组（group 字段），对不同组应用不同 RPM |
| Discord 身份组定期校验 | tasks 模块增加定时任务，调 Discord API 批量检查用户身份组状态 |
| Discord Bot 通知 | 封禁时通过 Bot 私信用户或发送到管理频道 |

---
