# NewAPI Guard

一个给 NewAPI 用的透明外挂守卫层，负责 Discord 准入、请求风控、签到拦截和运维面板，且不修改 NewAPI 源码。

## 项目定位

Guard 不是替换 NewAPI，而是作为它前面的外部协同服务存在。

它通过以下方式和 NewAPI 集成：

- OpenResty 路由分发
- NewAPI 管理 API
- NewAPI 自定义 OAuth Provider
- Guard 自己的 SQLite 状态库

目标是让 NewAPI 保持可升级，同时补上它原生不提供的控制能力。

## 核心能力

- Discord 准入登录
- 基于服务器和身份组的访问规则
- `/v1/*` 透明代理和 UA / RPM 风控
- `/api/user/checkin` 签到接管
- 白名单、封禁、配置、日志、统计的管理面板
- 不侵入 NewAPI 业务代码

## 设计原则

- 不侵入：NewAPI 不改源码
- 透明：用户仍然只接触 NewAPI 界面
- 可升级：NewAPI 可以独立更新
- 轻量：Go + SQLite + 现有反向代理
- 解耦：认证、代理、管理逻辑彼此独立

## 架构概览

```text
用户
  |
  v
OpenResty
  |
  +-- /v1/*             -> Guard
  +-- /api/user/checkin -> Guard
  +-- /guard/*          -> Guard
  +-- /*                -> NewAPI

Guard
  - Discord 准入
  - NewAPI 自定义 OAuth Provider
  - 请求风控代理
  - 管理面板
  - SQLite 状态

NewAPI
  - 原生前端
  - 原生 session
  - 原生管理 API
  - 原生 custom OAuth provider
```

## 登录流程

推荐流程是：

1. NewAPI 登录页显示自定义 OAuth 按钮。
2. 用户点击后跳转到 Guard 的 `authorize` 接口。
3. Guard 去做 Discord OAuth。
4. Guard 校验服务器和身份组规则。
5. Guard 返回标准 OAuth 的 `code / token / userinfo`。
6. NewAPI 按自己的原生流程完成登录并写入 session。

这样可以把 Discord 准入控制交给 Guard，同时保留 NewAPI 原生登录态。

## 为什么不直接改 NewAPI

- NewAPI 已经支持自定义 OAuth Provider，Guard 可以直接接进去。
- NewAPI 前端资源是嵌入式构建产物，不适合作为主方案去硬改。
- 前端脚本注入只适合作为可选补丁，不应成为主登录链路。

## 当前状态

这个仓库目前以技术方案为主。

文档里已经定义了：

- 数据模型
- 路由方式
- OAuth 接入方式
- 代理风控逻辑
- 管理面板范围
- 部署结构

后续可以按文档逐步落地实现。

## 仓库内容

- `README.md`：项目简介
- 根目录技术文档：完整设计说明

## 计划技术栈

- 后端：Go
- 前端：Vue 3 + Naive UI
- 数据库：SQLite
- 反向代理：OpenResty
- 部署：Docker Compose

## 范围说明

- Guard 不替代 NewAPI
- Guard 只补 NewAPI 不提供的控制层能力
- 签到页文案微调属于超低优先级项，不影响主线开发

## License

MIT
