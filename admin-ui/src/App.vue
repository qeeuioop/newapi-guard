<template>
  <n-config-provider :theme="darkTheme" :theme-overrides="themeOverrides">
    <template v-if="!auth.token">
      <div class="login-shell">
        <div class="login-panel">
          <section class="frame login-hero">
            <div>
              <div class="hero-eyebrow">公益站管理员控制台</div>
              <h1 class="hero-title">Guard 高级控制台</h1>
              <p class="hero-copy">
                面向公益站管理员的高密度值守界面。这里不追求花哨，而是把用户、封禁、白名单、日志和代理设置收拢到同一处，方便快速判断和快速处置。
              </p>
            </div>

            <div class="metric-row">
              <article class="metric-tile">
                <span>封禁口径</span>
                <strong>status = 2</strong>
              </article>
              <article class="metric-tile">
                <span>限速口径</span>
                <strong>用户级 RPM</strong>
              </article>
              <article class="metric-tile">
                <span>代理目标</span>
                <strong>可配置</strong>
              </article>
            </div>

            <div class="signal-grid">
              <article class="signal-card">
                <div class="capsule-label">核心原则</div>
                <div class="signal-value">New API 负责用户真相源</div>
                <div class="signal-note">Guard 只补充违规原因、时间、UA、IP 和操作上下文。</div>
              </article>
              <article class="signal-card">
                <div class="capsule-label">值守方向</div>
                <div class="signal-value">冷静 / 精准 / 克制</div>
                <div class="signal-note">适合长时间值守和快速交接，不做模板味很重的通用后台。</div>
              </article>
            </div>
          </section>

          <section class="frame login-card">
            <div>
              <div class="panel-label">登录</div>
              <h2 class="panel-title">进入控制台</h2>
              <p class="panel-copy">输入管理员密码后即可接通所有 `/guard/api/*` 后端接口。</p>
            </div>

            <n-form @submit.prevent>
              <n-form-item label="管理员密码">
                <n-input
                  v-model:value="login.password"
                  type="password"
                  show-password-on="click"
                  placeholder="请输入控制台密码"
                  @keyup.enter="submitLogin"
                />
              </n-form-item>
            </n-form>

            <div class="signal-grid compact-signals">
              <article class="signal-card">
                <div class="capsule-label">后端兼容</div>
                <div class="signal-note">继续使用现有 `/guard/admin/` 和 `/guard/api/*`，无需改后端路径。</div>
              </article>
              <article class="signal-card">
                <div class="capsule-label">会话提示</div>
                <div class="signal-note">{{ login.notice || "登录后会保留本地会话，并在手动退出时通知后端失效。" }}</div>
              </article>
            </div>

            <div class="auth-row">
              <n-button type="primary" size="large" :loading="authLoading" @click="submitLogin">登录控制台</n-button>
            </div>
          </section>
        </div>
      </div>
    </template>

    <template v-else>
      <div class="tool-shell">
        <div class="tool-layout">
          <aside class="nav-frame">
            <div class="brand-stack">
              <div class="hero-eyebrow">公益站守卫层</div>
              <div class="brand-title">Guard</div>
              <div class="brand-chip-row">
                <div class="brand-chip"><strong>{{ dashboard.active_bans || 0 }}</strong> 当前封禁</div>
                <div class="brand-chip"><strong>{{ dashboard.whitelist_count || 0 }}</strong> 白名单</div>
              </div>
            </div>

            <nav class="nav-list">
              <button
                v-for="item in navItems"
                :key="item.key"
                class="nav-button"
                :class="{ active: activeView === item.key }"
                @click="openView(item.key)"
              >
                <div>
                  <strong>{{ item.label }}</strong>
                  <span>{{ item.hint }}</span>
                </div>
                <div class="nav-count">{{ navCount(item.key) }}</div>
              </button>
            </nav>

            <div class="nav-foot">
              <div class="capsule-label">同步口径</div>
              <p class="muted">
                封禁以 <span class="mono">status = 2</span> 为准，RPM 以用户为维度累计，代理目标统一取自
                <span class="mono">newapi_base_url</span>。
              </p>
            </div>
          </aside>

          <main class="main-frame">
            <header class="main-head">
              <div>
                <div class="section-kicker">{{ sectionMeta.kicker }}</div>
                <h2 class="section-title">{{ sectionMeta.title }}</h2>
                <p class="section-copy">{{ sectionMeta.copy }}</p>
              </div>
              <div class="main-actions">
                <n-button secondary @click="refreshCurrentView">刷新当前视图</n-button>
                <n-button tertiary @click="refreshAll">全量同步</n-button>
                <n-button type="warning" ghost @click="openBanModal()">处理封禁</n-button>
                <n-button quaternary @click="logout">退出</n-button>
              </div>
            </header>

            <n-spin :show="pageLoading">
              <div class="content-scroll">
                <section class="section-stack section-top">
                  <div class="status-strip">
                    <article class="status-chip">
                      <div class="capsule-label">当前状态</div>
                      <div class="status-value">{{ currentViewHealth.label }}</div>
                      <div class="status-note">{{ currentViewHealth.note }}</div>
                    </article>
                    <article class="status-chip">
                      <div class="capsule-label">透明代理目标</div>
                      <div class="status-value mono">{{ settingsModel.newapi_base_url || "未配置" }}</div>
                      <div class="status-note">统一影响透明代理目标和 Guard 对 New API 的内部调用。</div>
                    </article>
                    <article class="status-chip">
                      <div class="capsule-label">用户级 RPM</div>
                      <div class="status-value">{{ settingsModel.rpm_limit || "—" }} / 分钟</div>
                      <div class="status-note">请求头鉴权中的 key 会先映射到用户，再进行分钟级限速。</div>
                    </article>
                    <article class="status-chip">
                      <div class="capsule-label">最近同步</div>
                      <div class="status-value">{{ currentViewSyncedAt }}</div>
                      <div class="status-note">只刷新当前视图时不会影响其它页的缓存状态。</div>
                    </article>
                  </div>

                  <div v-if="currentViewError" class="alert-shell">
                    <n-alert type="error" title="当前视图最近一次同步失败" :show-icon="false">
                      {{ currentViewError }}
                    </n-alert>
                  </div>
                </section>

                <section v-if="activeView === 'dashboard'" class="section-stack">
                  <div class="summary-grid">
                    <article class="stat-card">
                      <div class="capsule-label">今日请求</div>
                      <div class="stat-value">{{ formatNumber(dashboard.today?.total_requests) }}</div>
                      <div class="stat-foot">
                        <span>来自透明代理链路</span>
                        <n-tag size="small" :bordered="false">/v1/*</n-tag>
                      </div>
                    </article>
                    <article class="stat-card">
                      <div class="capsule-label">当前封禁</div>
                      <div class="stat-value">{{ formatNumber(dashboard.active_bans) }}</div>
                      <div class="stat-foot">
                        <span>以 New API 状态为准</span>
                        <n-tag size="small" type="warning" :bordered="false">status = 2</n-tag>
                      </div>
                    </article>
                    <article class="stat-card">
                      <div class="capsule-label">白名单</div>
                      <div class="stat-value">{{ formatNumber(dashboard.whitelist_count) }}</div>
                      <div class="stat-foot">
                        <span>豁免 UA 与 RPM</span>
                        <n-tag size="small" type="success" :bordered="false">Bypass</n-tag>
                      </div>
                    </article>
                    <article class="stat-card">
                      <div class="capsule-label">新增封禁</div>
                      <div class="stat-value">{{ formatNumber(dashboard.today?.new_bans) }}</div>
                      <div class="stat-foot">
                        <span>记录在 Guard 上下文表</span>
                        <n-tag size="small" type="error" :bordered="false">Risk</n-tag>
                      </div>
                    </article>
                  </div>

                  <div class="overview-grid">
                    <article class="overview-block">
                      <h3 class="block-title">今日风险面板</h3>
                      <div class="signal-line">
                        <div>
                          <div class="signal-name">UA 拦截次数</div>
                          <div class="signal-note">累计触发不受信客户端识别</div>
                        </div>
                        <div class="signal-value">{{ formatNumber(dashboard.today?.blocked_ua) }}</div>
                      </div>
                      <div class="signal-line">
                        <div>
                          <div class="signal-name">RPM 拦截次数</div>
                          <div class="signal-note">按用户维度执行分钟级速率控制</div>
                        </div>
                        <div class="signal-value">{{ formatNumber(dashboard.today?.blocked_rpm) }}</div>
                      </div>
                      <div class="signal-line">
                        <div>
                          <div class="signal-name">签到完成次数</div>
                          <div class="signal-note">额度补发由 Guard 接管并落日志</div>
                        </div>
                        <div class="signal-value">{{ formatNumber(dashboard.today?.checkins) }}</div>
                      </div>
                      <div class="signal-line">
                        <div>
                          <div class="signal-name">新增用户</div>
                          <div class="signal-note">由管理员创建或代理自动补录上下文</div>
                        </div>
                        <div class="signal-value">{{ formatNumber(dashboard.today?.new_users) }}</div>
                      </div>
                    </article>

                    <article class="rail-card">
                      <h3 class="block-title">快速动作</h3>
                      <div class="quick-grid">
                        <n-button type="primary" @click="openCreateUserModal">创建用户</n-button>
                        <n-button secondary @click="openView('users')">搜索用户</n-button>
                        <n-button secondary @click="openView('settings')">调整设置</n-button>
                        <n-button secondary @click="openView('bans')">查看封禁</n-button>
                      </div>
                      <n-divider />
                      <div class="mini-grid">
                        <div class="tiny-chip"><strong>{{ settingsModel.newapi_base_url || "未配置" }}</strong></div>
                        <div class="tiny-chip"><strong>{{ settingsModel.public_base_url || "自动推断" }}</strong></div>
                      </div>
                      <p class="muted">当前控制台重点围绕用户级 RPM、Discord 准入、封禁上下文与公益站日常值守展开。</p>
                    </article>
                  </div>

                  <div class="panel-grid">
                    <article class="overview-block">
                      <div class="panel-heading">
                        <div>
                          <h3 class="block-title">当前封禁预览</h3>
                          <p class="muted">优先展示仍处于封禁状态的用户，帮助管理员快速判断是否需要解除。</p>
                        </div>
                        <n-button text @click="openView('bans')">进入封禁视图</n-button>
                      </div>
                      <div v-if="activeBans.length" class="section-stack">
                        <div v-for="item in activeBans.slice(0, 4)" :key="'dash-ban-' + item.newapi_user_id" class="list-row">
                          <div class="cell-stack">
                            <div class="cell-title">{{ item.display_name || item.username || "用户 " + item.newapi_user_id }}</div>
                            <div class="cell-sub">ID {{ item.newapi_user_id }} · {{ item.reason || "无上下文" }}</div>
                          </div>
                          <n-button size="small" secondary @click="quickUnban(item)">解除</n-button>
                        </div>
                      </div>
                      <div v-else class="empty-shell">
                        <n-empty description="当前没有活跃封禁用户" />
                      </div>
                    </article>

                    <article class="overview-block">
                      <div class="panel-heading">
                        <div>
                          <h3 class="block-title">统计趋势预览</h3>
                          <p class="muted">最近几天的请求量与风险事件，用于快速判断站点是否进入异常波动区间。</p>
                        </div>
                        <n-button text @click="openView('logs')">查看日志</n-button>
                      </div>
                      <div v-if="statsLogs.length">
                        <div v-for="item in statsLogs.slice(0, 5)" :key="'trend-' + item.date" class="trend-row">
                          <div>
                            <div class="trend-date">{{ item.date }}</div>
                            <div class="cell-sub">UA {{ item.blocked_ua }} · RPM {{ item.blocked_rpm }} · 封禁 {{ item.new_bans }}</div>
                          </div>
                          <div class="trend-right">
                            <div class="trend-bar"><span :style="{ width: trendWidth(item.total_requests) }"></span></div>
                            <div class="trend-value">{{ formatNumber(item.total_requests) }}</div>
                          </div>
                        </div>
                      </div>
                      <div v-else class="empty-shell">
                        <n-empty description="还没有统计样本" />
                      </div>
                    </article>
                  </div>
                </section>

                <section v-else-if="activeView === 'users'" class="section-stack">
                  <div class="filters">
                    <n-input
                      v-model:value="userSearchDraft"
                      clearable
                      placeholder="搜索 newapi id、discord id、discord 名称或用户名"
                      @keyup.enter="applyUserSearch"
                    />
                    <n-button type="primary" @click="applyUserSearch">搜索</n-button>
                    <n-button tertiary @click="resetUserSearch">清空</n-button>
                    <n-button secondary @click="openCreateUserModal">创建用户</n-button>
                  </div>

                  <div class="toolbar-split soft-card">
                    <div class="table-meta">
                      {{ userQuery.search ? "当前关键字：" + userQuery.search : "当前未使用关键字过滤，可直接浏览最近同步到的用户页。" }}
                      · 共 {{ formatNumber(userQuery.total || users.length) }} 条 · 第 {{ userQuery.page }} / {{ userPageCount }} 页
                    </div>
                    <div class="table-actions">
                      <n-select style="width: 132px" :value="userQuery.size" :options="userPageSizeOptions" @update:value="handleUserPageSizeChange" />
                      <n-pagination :page="userQuery.page" :page-count="userPageCount" simple @update:page="handleUserPageChange" />
                    </div>
                  </div>

                  <article class="overview-block">
                    <div class="panel-heading">
                      <div>
                        <h3 class="block-title">用户列表</h3>
                        <p class="muted">搜索时优先走 New API 实时用户数据，同时保留 Guard 记录的 Discord 绑定与白名单信息。</p>
                      </div>
                      <n-tag size="large" :bordered="false">{{ users.length }} 条结果</n-tag>
                    </div>

                    <div class="table-shell" v-if="users.length">
                      <table>
                        <thead>
                          <tr>
                            <th>用户</th>
                            <th>配额 / 状态</th>
                            <th>身份映射</th>
                            <th>操作</th>
                          </tr>
                        </thead>
                        <tbody>
                          <tr v-for="item in users" :key="'user-' + item.newapi_user_id">
                            <td>
                              <div class="cell-stack">
                                <div class="cell-title">{{ item.display_name || item.username || "用户 " + item.newapi_user_id }}</div>
                                <div class="cell-sub mono">newapi {{ item.newapi_user_id }}</div>
                                <div class="cell-sub">{{ item.email || "未绑定邮箱" }}</div>
                              </div>
                            </td>
                            <td>
                              <div class="cell-stack">
                                <div class="cell-title">{{ formatNumber(item.quota) }}</div>
                                <div class="cell-sub">状态：{{ item.status === 2 ? "已封禁" : "正常" }}</div>
                                <div class="cell-sub">创建：{{ parseDate(item.created_at || item.created_at_unix) }}</div>
                              </div>
                            </td>
                            <td>
                              <div class="cell-stack">
                                <div class="cell-sub">Discord：{{ item.discord_name || item.discord_id || "未绑定" }}</div>
                                <div class="cell-sub">白名单：{{ item.is_whitelist ? "是" : "否" }}</div>
                                <div class="cell-sub">分组：{{ item.group || "default" }}</div>
                              </div>
                            </td>
                            <td>
                              <div class="action-row">
                                <n-button size="small" secondary @click="toggleWhitelist(item)">{{ item.is_whitelist ? "移出白名单" : "加入白名单" }}</n-button>
                                <n-button size="small" type="warning" ghost @click="openBanModal(item)">{{ item.status === 2 ? "再次记录封禁" : "封禁" }}</n-button>
                                <n-button v-if="item.status === 2" size="small" type="success" ghost @click="quickUnban(item)">解除封禁</n-button>
                              </div>
                            </td>
                          </tr>
                        </tbody>
                      </table>
                    </div>
                    <div v-else class="empty-shell">
                      <n-empty description="暂时没有匹配的用户">
                        <template #extra>
                          <div class="action-row">
                            <n-button tertiary @click="resetUserSearch">清空筛选</n-button>
                            <n-button secondary @click="openCreateUserModal">创建用户</n-button>
                          </div>
                        </template>
                      </n-empty>
                    </div>
                  </article>
                </section>

                <section v-else-if="activeView === 'whitelist'" class="section-stack">
                  <article class="overview-block">
                    <div class="panel-heading">
                      <div>
                        <h3 class="block-title">白名单用户</h3>
                        <p class="muted">白名单用户会跳过 UA 与 RPM 检查，但仍会正常扣减额度。添加入口也保留在用户列表里，便于统一检索后再授权。</p>
                      </div>
                      <n-tag size="large" type="success" :bordered="false">{{ whitelist.length }} 人</n-tag>
                    </div>
                    <div class="table-shell" v-if="whitelist.length">
                      <table>
                        <thead>
                          <tr>
                            <th>身份</th>
                            <th>来源</th>
                            <th>加入时间</th>
                            <th>操作</th>
                          </tr>
                        </thead>
                        <tbody>
                          <tr v-for="item in whitelist" :key="'whitelist-' + item.newapi_user_id">
                            <td>
                              <div class="cell-stack">
                                <div class="cell-title">{{ item.discord_name || item.discord_id || "用户 " + item.newapi_user_id }}</div>
                                <div class="cell-sub mono">newapi {{ item.newapi_user_id }}</div>
                              </div>
                            </td>
                            <td>
                              <div class="cell-stack">
                                <div class="cell-sub">白名单豁免 UA 与 RPM</div>
                                <div class="cell-sub">推荐配合用户页的搜索与封禁状态一起判断</div>
                              </div>
                            </td>
                            <td class="cell-sub">{{ parseDate(item.created_at) }}</td>
                            <td>
                              <n-button size="small" type="warning" ghost @click="toggleWhitelist(item)">移出白名单</n-button>
                            </td>
                          </tr>
                        </tbody>
                      </table>
                    </div>
                    <div v-else class="empty-shell">
                      <n-empty description="当前没有白名单用户" />
                    </div>
                  </article>
                </section>

                <section v-else-if="activeView === 'bans'" class="section-stack">
                  <div class="filters">
                    <n-button :type="banMode === 'active' ? 'primary' : 'default'" @click="switchBanMode('active')">当前封禁</n-button>
                    <n-button :type="banMode === 'all' ? 'primary' : 'default'" @click="switchBanMode('all')">封禁历史</n-button>
                    <n-button type="warning" ghost @click="openBanModal()">新增封禁</n-button>
                    <n-input v-model:value="banFilter" clearable placeholder="按用户、Discord、原因或 UA 过滤当前列表" class="ban-filter" />
                  </div>

                  <div class="signal-grid">
                    <article class="signal-card">
                      <div class="capsule-label">当前封禁</div>
                      <div class="signal-value">{{ formatNumber(activeBans.length) }}</div>
                      <div class="signal-note">直接以 New API 的 status=2 为准。</div>
                    </article>
                    <article class="signal-card">
                      <div class="capsule-label">含 Guard 上下文</div>
                      <div class="signal-value">{{ formatNumber(activeBansWithContext) }}</div>
                      <div class="signal-note">可直接看到原因、UA、IP 与封禁时间。</div>
                    </article>
                    <article class="signal-card">
                      <div class="capsule-label">历史记录</div>
                      <div class="signal-value">{{ formatNumber(banHistory.length) }}</div>
                      <div class="signal-note">用于审计和回溯，不代表当前状态。</div>
                    </article>
                    <article class="signal-card">
                      <div class="capsule-label">本地过滤</div>
                      <div class="signal-value">{{ banFilter || "未启用" }}</div>
                      <div class="signal-note">只在当前视图内筛选，不影响后端查询口径。</div>
                    </article>
                  </div>

                  <article class="overview-block">
                    <div class="panel-heading">
                      <div>
                        <h3 class="block-title">{{ banMode === "active" ? "当前封禁列表" : "封禁历史记录" }}</h3>
                        <p v-if="banMode === 'active'" class="muted">当前视图直接以 New API 的用户状态为准，再叠加 Guard 记录的原因、到期时间、最近违规 UA 等上下文。</p>
                        <p v-else class="muted">历史视图保留 Guard 记录下来的封禁事件，用于审计时间线与追溯原因。</p>
                      </div>
                      <n-tag size="large" :bordered="false">{{ filteredBans.length }} / {{ displayedBans.length }} 条</n-tag>
                    </div>

                    <div class="table-shell" v-if="filteredBans.length">
                      <table>
                        <thead>
                          <tr>
                            <th>用户</th>
                            <th>封禁上下文</th>
                            <th>时间与状态</th>
                            <th>操作</th>
                          </tr>
                        </thead>
                        <tbody>
                          <tr v-for="item in filteredBans" :key="'ban-' + (item.id || item.newapi_user_id)">
                            <td>
                              <div class="cell-stack">
                                <div class="cell-title">{{ item.display_name || item.username || "用户 " + item.newapi_user_id }}</div>
                                <div class="cell-sub mono">newapi {{ item.newapi_user_id }}</div>
                                <div class="cell-sub">Discord：{{ item.discord_name || item.discord_id || "未记录" }}</div>
                              </div>
                            </td>
                            <td>
                              <div class="cell-stack">
                                <div class="cell-title">{{ item.reason || "无上下文" }}</div>
                                <div class="cell-sub">违规 UA：{{ item.violation_ua || "未记录" }}</div>
                                <div class="cell-sub">客户端 IP：{{ item.client_ip || "未记录" }}</div>
                              </div>
                            </td>
                            <td>
                              <div class="cell-stack">
                                <div class="cell-sub">封禁时间：{{ parseDate(item.created_at) }}</div>
                                <div class="cell-sub">到期时间：{{ parseDate(item.expire_at) }}</div>
                                <div class="cell-sub">{{ item.context_missing ? "直接来自 New API 状态" : "含 Guard 上下文" }}</div>
                              </div>
                            </td>
                            <td>
                              <div class="action-row">
                                <n-button v-if="banMode === 'active'" size="small" type="success" ghost @click="quickUnban(item)">解除封禁</n-button>
                                <n-button v-else size="small" secondary @click="openBanModal(item)">再次封禁</n-button>
                              </div>
                            </td>
                          </tr>
                        </tbody>
                      </table>
                    </div>
                    <div v-else class="empty-shell">
                      <n-empty :description="banFilter ? '没有匹配当前过滤条件的封禁项' : (banMode === 'active' ? '当前没有封禁用户' : '暂无封禁历史')">
                        <template #extra>
                          <n-button tertiary @click="banFilter = ''">清除过滤</n-button>
                        </template>
                      </n-empty>
                    </div>
                  </article>
                </section>

                <section v-else-if="activeView === 'settings'" class="section-stack">
                  <div class="settings-grid">
                    <article class="overview-block settings-stack">
                      <div>
                        <h3 class="block-title">代理与运维设置</h3>
                        <p class="muted">这一组设置决定透明代理目标、管理员凭据、签到策略与访问频率控制，是整条 Guard 主链路的核心。</p>
                      </div>
                      <div class="compact-grid">
                        <n-form-item label="New API 内网地址">
                          <n-input v-model:value="settingsModel.newapi_base_url" placeholder="http://new-api:3000" />
                        </n-form-item>
                        <n-form-item label="New API 管理员令牌">
                          <n-input v-model:value="settingsModel.newapi_admin_token" type="password" show-password-on="click" placeholder="用于用户与封禁操作" />
                        </n-form-item>
                        <n-form-item label="控制台公开地址">
                          <n-input v-model:value="settingsModel.public_base_url" placeholder="可留空自动推断" />
                        </n-form-item>
                        <n-form-item label="管理员密码">
                          <n-input v-model:value="settingsModel.admin_password" type="password" show-password-on="click" placeholder="控制台登录密码" />
                        </n-form-item>
                        <n-form-item label="用户级 RPM 限制">
                          <n-input-number v-model:value="settingsModel.rpm_limit" :min="1" />
                        </n-form-item>
                        <n-form-item label="UA 违规封禁阈值">
                          <n-input-number v-model:value="settingsModel.ua_ban_strikes" :min="1" />
                        </n-form-item>
                        <n-form-item label="签到补发额度">
                          <n-input-number v-model:value="settingsModel.checkin_quota" :min="0" />
                        </n-form-item>
                        <n-form-item label="签到余额阈值">
                          <n-input-number v-model:value="settingsModel.checkin_threshold" :min="0" />
                        </n-form-item>
                      </div>
                      <n-form-item label="允许的 UA 前缀">
                        <n-input v-model:value="settingsText.allowed_ua" type="textarea" :autosize="{ minRows: 5, maxRows: 8 }" placeholder="每行一个前缀，例如 FLClash/" />
                      </n-form-item>
                    </article>

                    <article class="overview-block settings-stack">
                      <div>
                        <h3 class="block-title">OAuth 与 Discord 准入</h3>
                        <p class="muted">这里控制 Guard 作为 OAuth Provider 暴露给 New API 的能力，同时维护 Discord 准入规则和相关凭据。</p>
                      </div>
                      <div class="compact-grid">
                        <n-form-item label="OAuth Client ID">
                          <n-input v-model:value="settingsModel.oauth_client_id" />
                        </n-form-item>
                        <n-form-item label="OAuth Client Secret">
                          <n-input v-model:value="settingsModel.oauth_client_secret" type="password" show-password-on="click" />
                        </n-form-item>
                        <n-form-item label="Provider Slug">
                          <n-input v-model:value="settingsModel.oauth_provider_slug" />
                        </n-form-item>
                        <n-form-item label="Discord Client ID">
                          <n-input v-model:value="settingsModel.discord_client_id" />
                        </n-form-item>
                        <n-form-item label="Discord Client Secret">
                          <n-input v-model:value="settingsModel.discord_client_secret" type="password" show-password-on="click" />
                        </n-form-item>
                        <n-form-item label="Discord Guild ID">
                          <n-input v-model:value="settingsModel.discord_guild_id" />
                        </n-form-item>
                        <n-form-item label="State TTL（秒）">
                          <n-input-number v-model:value="settingsModel.oauth_state_ttl_seconds" :min="60" />
                        </n-form-item>
                        <n-form-item label="Code TTL（秒）">
                          <n-input-number v-model:value="settingsModel.oauth_code_ttl_seconds" :min="60" />
                        </n-form-item>
                        <n-form-item label="Token TTL（秒）">
                          <n-input-number v-model:value="settingsModel.oauth_token_ttl_seconds" :min="60" />
                        </n-form-item>
                      </div>
                      <n-form-item label="Discord OAuth Scopes">
                        <n-input v-model:value="settingsText.discord_oauth_scopes" type="textarea" :autosize="{ minRows: 3, maxRows: 5 }" placeholder="每行一个 scope，例如 identify" />
                      </n-form-item>
                      <n-form-item label="Discord 准入规则 JSON">
                        <n-input v-model:value="settingsText.discord_access_policy" type="textarea" :autosize="{ minRows: 10, maxRows: 18 }" placeholder='{"logic":"and","conditions":[],"groups":[]}' />
                      </n-form-item>
                      <div class="field-caption">准入规则支持嵌套条件组。建议保存 guild_id 与 role_id，不依赖名称，避免 Discord 改名造成规则失效。</div>
                    </article>
                  </div>

                  <article class="overview-block">
                    <div class="panel-heading">
                      <div>
                        <h3 class="block-title">设置说明</h3>
                        <p class="muted">配置保存后会立即写入 Guard 本地数据库。代理目标、OAuth 凭据、签到阈值与准入规则都会在运行中生效。</p>
                      </div>
                      <div class="action-row">
                        <n-button secondary @click="resetSettingsDraft">重置草稿</n-button>
                        <n-button type="primary" @click="saveSettings">保存设置</n-button>
                      </div>
                    </div>
                    <div class="section-note">
                      为避免误操作，建议先确认 <span class="mono">newapi_base_url</span>、<span class="mono">newapi_admin_token</span> 与
                      <span class="mono">public_base_url</span> 三项配置可用，再修改 Discord 准入或 OAuth 相关字段。
                    </div>
                  </article>
                </section>

                <section v-else-if="activeView === 'logs'" class="section-stack">
                  <article class="overview-block">
                    <div class="panel-heading">
                      <div>
                        <h3 class="block-title">运维日志</h3>
                        <p class="muted">把封禁、签到和每日统计拆成三个观察面，便于管理员在同一页里做审计与趋势判断。</p>
                      </div>
                      <n-button secondary @click="refreshLogsOnly">刷新日志</n-button>
                    </div>

                    <n-tabs type="line" animated v-model:value="logView">
                      <n-tab-pane name="bans" tab="封禁日志">
                        <div class="table-shell" v-if="banLogs.length">
                          <table>
                            <thead>
                              <tr>
                                <th>用户</th>
                                <th>原因</th>
                                <th>违规信息</th>
                                <th>时间</th>
                              </tr>
                            </thead>
                            <tbody>
                              <tr v-for="item in banLogs" :key="'ban-log-' + item.id">
                                <td>
                                  <div class="cell-stack">
                                    <div class="cell-title mono">{{ item.newapi_user_id }}</div>
                                    <div class="cell-sub">{{ item.discord_id || "未记录 Discord" }}</div>
                                  </div>
                                </td>
                                <td>
                                  <div class="cell-stack">
                                    <div class="cell-title">{{ item.reason }}</div>
                                    <div class="cell-sub">时长：{{ item.duration }}</div>
                                  </div>
                                </td>
                                <td>
                                  <div class="cell-stack">
                                    <div class="cell-sub">UA：{{ item.violation_ua || "未记录" }}</div>
                                    <div class="cell-sub">IP：{{ item.client_ip || "未记录" }}</div>
                                  </div>
                                </td>
                                <td>
                                  <div class="cell-stack">
                                    <div class="cell-sub">封禁：{{ parseDate(item.created_at) }}</div>
                                    <div class="cell-sub">解除：{{ parseDate(item.unbanned_at) }}</div>
                                  </div>
                                </td>
                              </tr>
                            </tbody>
                          </table>
                        </div>
                        <div v-else class="empty-shell">
                          <n-empty description="暂无封禁日志" />
                        </div>
                      </n-tab-pane>

                      <n-tab-pane name="checkins" tab="签到日志">
                        <div class="filters">
                          <n-input v-model:value="checkinFilter" clearable placeholder="按 newapi 用户 ID 过滤" @keyup.enter="loadCheckinLogsAction" />
                          <n-button secondary @click="loadCheckinLogsAction">应用过滤</n-button>
                        </div>
                        <div class="table-shell" v-if="checkinLogs.length">
                          <table>
                            <thead>
                              <tr>
                                <th>用户</th>
                                <th>补发额度</th>
                                <th>签到日期</th>
                              </tr>
                            </thead>
                            <tbody>
                              <tr v-for="item in checkinLogs" :key="'checkin-log-' + item.id">
                                <td class="mono">{{ item.newapi_user_id }}</td>
                                <td>{{ formatNumber(item.quota_added) }}</td>
                                <td>{{ item.checked_at }}</td>
                              </tr>
                            </tbody>
                          </table>
                        </div>
                        <div v-else class="empty-shell">
                          <n-empty description="暂无签到日志" />
                        </div>
                      </n-tab-pane>

                      <n-tab-pane name="stats" tab="每日统计">
                        <div class="table-shell" v-if="statsLogs.length">
                          <table>
                            <thead>
                              <tr>
                                <th>日期</th>
                                <th>请求量</th>
                                <th>风险事件</th>
                                <th>运营数据</th>
                              </tr>
                            </thead>
                            <tbody>
                              <tr v-for="item in statsLogs" :key="'stats-log-' + item.date">
                                <td>{{ item.date }}</td>
                                <td>{{ formatNumber(item.total_requests) }}</td>
                                <td>
                                  <div class="cell-stack">
                                    <div class="cell-sub">UA 拦截：{{ item.blocked_ua }}</div>
                                    <div class="cell-sub">RPM 拦截：{{ item.blocked_rpm }}</div>
                                  </div>
                                </td>
                                <td>
                                  <div class="cell-stack">
                                    <div class="cell-sub">签到：{{ item.checkins }}</div>
                                    <div class="cell-sub">新增用户：{{ item.new_users }} · 新增封禁：{{ item.new_bans }}</div>
                                  </div>
                                </td>
                              </tr>
                            </tbody>
                          </table>
                        </div>
                        <div v-else class="empty-shell">
                          <n-empty description="暂无统计数据" />
                        </div>
                      </n-tab-pane>
                    </n-tabs>
                  </article>
                </section>
              </div>
            </n-spin>
          </main>
        </div>
      </div>

      <n-modal v-model:show="createUserModal.show" preset="card" style="width:min(720px, 92vw)" title="创建用户">
        <div class="settings-stack">
          <div class="filters">
            <n-button :type="createUserModal.form.mode === 'password' ? 'primary' : 'default'" @click="createUserModal.form.mode = 'password'">账号密码</n-button>
            <n-button :type="createUserModal.form.mode === 'discord' ? 'primary' : 'default'" @click="createUserModal.form.mode = 'discord'">Discord 绑定</n-button>
          </div>
          <div class="compact-grid" v-if="createUserModal.form.mode === 'password'">
            <n-form-item label="用户名">
              <n-input v-model:value="createUserModal.form.username" placeholder="例如 tom" />
            </n-form-item>
            <n-form-item label="密码">
              <n-input v-model:value="createUserModal.form.password" type="password" show-password-on="click" />
            </n-form-item>
          </div>
          <div class="compact-grid" v-else>
            <n-form-item label="Discord ID">
              <n-input v-model:value="createUserModal.form.discord_id" placeholder="例如 298374928374" />
            </n-form-item>
            <n-form-item label="Discord 名称">
              <n-input v-model:value="createUserModal.form.discord_name" placeholder="例如 Tom#1234" />
            </n-form-item>
          </div>
          <div class="compact-grid">
            <n-form-item label="初始额度">
              <n-input-number v-model:value="createUserModal.form.initial_quota" :min="0" />
            </n-form-item>
            <n-form-item label="是否加入白名单">
              <n-switch v-model:value="createUserModal.form.is_whitelist" />
            </n-form-item>
          </div>
          <div class="action-row">
            <n-button secondary @click="createUserModal.show = false">取消</n-button>
            <n-button type="primary" :loading="createUserModal.loading" @click="submitCreateUser">确认创建</n-button>
          </div>
        </div>
      </n-modal>

      <n-modal v-model:show="banModal.show" preset="card" style="width:min(760px, 92vw)" title="处理封禁">
        <div class="settings-stack">
          <div class="section-note">
            你可以直接输入 <span class="mono">newapi 用户 ID</span>、<span class="mono">Discord ID</span>，或者让表格中的行操作自动带入当前用户。
          </div>
          <div class="compact-grid">
            <n-form-item label="统一用户标识">
              <n-input v-model:value="banModal.form.user_ref" placeholder="例如 123 或 298374928374" />
            </n-form-item>
            <n-form-item label="Discord ID（可选）">
              <n-input v-model:value="banModal.form.discord_id" placeholder="优先用于 Discord 映射查找" />
            </n-form-item>
            <n-form-item label="newapi 用户 ID（可选）">
              <n-input-number v-model:value="banModal.form.newapi_user_id" :min="1" />
            </n-form-item>
            <n-form-item label="封禁时长">
              <n-select v-model:value="banModal.form.duration" :options="banDurationOptions" />
            </n-form-item>
          </div>
          <n-form-item label="封禁原因">
            <n-input v-model:value="banModal.form.reason" type="textarea" :autosize="{ minRows: 4, maxRows: 7 }" placeholder="记录操作原因，便于后续审计和交接" />
          </n-form-item>
          <div class="action-row">
            <n-button secondary @click="banModal.show = false">取消</n-button>
            <n-button type="warning" :loading="banModal.loading" @click="submitBan">确认封禁</n-button>
          </div>
        </div>
      </n-modal>
    </template>
  </n-config-provider>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from "vue";
import { createDiscreteApi, darkTheme } from "naive-ui";

const themeOverrides = {
  common: {
    primaryColor: "oklch(0.78 0.124 174)",
    primaryColorHover: "oklch(0.83 0.11 174)",
    primaryColorPressed: "oklch(0.72 0.11 174)",
    infoColor: "oklch(0.78 0.124 174)",
    successColor: "oklch(0.76 0.12 160)",
    warningColor: "oklch(0.8 0.12 78)",
    errorColor: "oklch(0.7 0.16 27)",
    bodyColor: "oklch(0.19 0.02 220)",
    cardColor: "oklch(0.235 0.022 220)",
    modalColor: "oklch(0.235 0.022 220)",
    popoverColor: "oklch(0.235 0.022 220)",
    borderColor: "rgba(255,255,255,0.08)",
    textColorBase: "oklch(0.92 0.01 220)",
    textColor2: "oklch(0.75 0.014 220)",
    fontFamily: "\"Noto Sans SC\", \"Segoe UI\", sans-serif",
    fontFamilyMono: "\"Chakra Petch\", sans-serif"
  },
  Button: {
    borderRadiusMedium: "14px",
    borderRadiusSmall: "12px"
  },
  Input: {
    color: "rgba(255,255,255,0.02)",
    borderRadius: "14px"
  }
};

const { message, dialog } = createDiscreteApi(["message", "dialog"], {
  configProviderProps: {
    theme: darkTheme,
    themeOverrides
  }
});

const navItems = [
  { key: "dashboard", label: "总览", hint: "站点态势与风险信号" },
  { key: "users", label: "用户", hint: "检索、创建、配额与白名单" },
  { key: "whitelist", label: "白名单", hint: "豁免用户清单" },
  { key: "bans", label: "封禁", hint: "当前封禁与历史记录" },
  { key: "settings", label: "设置", hint: "代理、签到、OAuth 与准入" },
  { key: "logs", label: "日志", hint: "封禁、签到与统计趋势" }
];

const sectionMap = {
  dashboard: {
    kicker: "系统总览",
    title: "总览与风险信号",
    copy: "先判断站点是否稳定，再进入具体用户、封禁或日志视图。这个页只回答一个问题：现在值不值得立刻干预。"
  },
  users: {
    kicker: "用户台",
    title: "用户检索与快速操作",
    copy: "优先用列表搜索锁定用户，再直接处理白名单、封禁和额度入口。这里既看 New API 实时状态，也看 Guard 补充的 Discord 映射。"
  },
  whitelist: {
    kicker: "信任通道",
    title: "白名单清单",
    copy: "查看当前被豁免 UA 与 RPM 控制的用户，适合维护长期可信客户端与特殊账号。"
  },
  bans: {
    kicker: "风控台",
    title: "封禁处置台",
    copy: "当前封禁视图直接对齐 New API 的 status=2，历史视图则保留 Guard 的审计上下文。"
  },
  settings: {
    kicker: "控制面",
    title: "系统设置与准入参数",
    copy: "这里集中维护代理目标、签到策略、管理员凭据、OAuth Provider 配置以及 Discord 准入规则。"
  },
  logs: {
    kicker: "时间线",
    title: "日志与趋势",
    copy: "把封禁、签到和每日统计放在同一观察面板内，方便回看行为与追踪异常变化。"
  }
};

const auth = reactive({
  token: localStorage.getItem("guard_token") || ""
});

const login = reactive({
  password: "",
  notice: ""
});

const authLoading = ref(false);
const pageLoading = ref(false);
const activeView = ref("dashboard");
const logView = ref("bans");
const banMode = ref("active");
const banFilter = ref("");
const checkinFilter = ref("");
const userSearchDraft = ref("");

const viewLoaded = reactive({
  dashboard: false,
  users: false,
  whitelist: false,
  bans: false,
  settings: false,
  logs: false
});

const viewState = reactive({
  dashboard: { error: "", syncedAt: 0 },
  users: { error: "", syncedAt: 0 },
  whitelist: { error: "", syncedAt: 0 },
  bans: { error: "", syncedAt: 0 },
  settings: { error: "", syncedAt: 0 },
  logs: { error: "", syncedAt: 0 }
});

const dashboard = reactive({
  today: {
    total_requests: 0,
    blocked_ua: 0,
    blocked_rpm: 0,
    checkins: 0,
    new_users: 0,
    new_bans: 0
  },
  total_users: 0,
  active_bans: 0,
  whitelist_count: 0
});

const userQuery = reactive({
  search: "",
  page: 1,
  size: 20,
  total: 0
});

const users = ref([]);
const whitelist = ref([]);
const activeBans = ref([]);
const banHistory = ref([]);
const banLogs = ref([]);
const checkinLogs = ref([]);
const statsLogs = ref([]);

const settingsModel = reactive({});
const settingsText = reactive({
  allowed_ua: "",
  discord_oauth_scopes: "",
  discord_access_policy: ""
});

const createUserModal = reactive({
  show: false,
  loading: false,
  form: {
    mode: "password",
    username: "",
    password: "",
    discord_id: "",
    discord_name: "",
    initial_quota: 0,
    is_whitelist: false
  }
});

const banModal = reactive({
  show: false,
  loading: false,
  form: {
    user_ref: "",
    newapi_user_id: null,
    discord_id: "",
    reason: "",
    duration: "permanent"
  }
});

const banDurationOptions = [
  { label: "永久封禁", value: "permanent" },
  { label: "7 天", value: "7d" },
  { label: "30 天", value: "30d" }
];

const userPageSizeOptions = [
  { label: "20 / 页", value: 20 },
  { label: "50 / 页", value: 50 },
  { label: "100 / 页", value: 100 }
];

const sectionMeta = computed(() => sectionMap[activeView.value]);
const displayedBans = computed(() => (banMode.value === "active" ? activeBans.value : banHistory.value));
const userPageCount = computed(() => Math.max(1, Math.ceil((Number(userQuery.total) || 0) / userQuery.size)));
const activeBansWithContext = computed(() => activeBans.value.filter((item) => !item.context_missing).length);
const currentViewError = computed(() => viewState[activeView.value].error);
const currentViewSyncedAt = computed(() => {
  const syncedAt = viewState[activeView.value].syncedAt;
  return syncedAt ? new Date(syncedAt).toLocaleString("zh-CN") : "尚未同步";
});
const currentViewHealth = computed(() => {
  if (viewState[activeView.value].error) {
    return {
      label: "需要复查",
      note: "当前视图最近一次同步失败，建议先刷新当前视图。"
    };
  }
  if (viewState[activeView.value].syncedAt) {
    return {
      label: "同步正常",
      note: "当前视图已接通后端接口，可以继续执行管理操作。"
    };
  }
  return {
    label: "等待加载",
    note: "当前视图还没有完成首次同步。"
  };
});

const filteredBans = computed(() => {
  const keyword = banFilter.value.trim().toLowerCase();
  if (!keyword) {
    return displayedBans.value;
  }
  return displayedBans.value.filter((item) =>
    [
      item.display_name,
      item.username,
      item.discord_id,
      item.discord_name,
      item.reason,
      item.violation_ua,
      item.client_ip,
      item.newapi_user_id
    ].some((value) => String(value || "").toLowerCase().includes(keyword))
  );
});

function clone(value, fallback) {
  return JSON.parse(JSON.stringify(value ?? fallback));
}

function parseDate(value) {
  if (!value) return "—";
  if (typeof value === "number") {
    return new Date(value * 1000).toLocaleString("zh-CN");
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString("zh-CN");
}

function formatNumber(value) {
  return new Intl.NumberFormat("zh-CN").format(Number(value || 0));
}

function formatMaybeArray(value) {
  return Array.isArray(value) ? value.join("\n") : "";
}

function formatMaybeJson(value) {
  if (typeof value === "string") {
    try {
      return JSON.stringify(JSON.parse(value), null, 2);
    } catch {
      return value;
    }
  }
  return JSON.stringify(value ?? {}, null, 2);
}

function navCount(key) {
  switch (key) {
    case "dashboard":
      return dashboard.active_bans || 0;
    case "users":
      return users.value.length;
    case "whitelist":
      return whitelist.value.length || dashboard.whitelist_count || 0;
    case "bans":
      return displayedBans.value.length;
    case "settings":
      return "Cfg";
    case "logs":
      return statsLogs.value.length || "Log";
    default:
      return 0;
  }
}

async function request(path, options = {}) {
  const headers = { ...(options.headers || {}) };
  if (auth.token) {
    headers.Authorization = `Bearer ${auth.token}`;
  }
  if (options.body !== undefined && !headers["Content-Type"]) {
    headers["Content-Type"] = "application/json";
  }

  const response = await fetch(path, {
    method: options.method || "GET",
    headers,
    body: options.body !== undefined ? JSON.stringify(options.body) : undefined
  });
  const data = await response.json().catch(() => ({}));

  if (response.status === 401 || data.message === "未授权") {
    await logout(true);
    throw new Error("登录状态已失效，请重新登录");
  }
  if (!response.ok) {
    throw new Error(data.message || `请求失败（${response.status}）`);
  }
  if (Object.prototype.hasOwnProperty.call(data, "success") && data.success === false) {
    throw new Error(data.message || "请求失败");
  }
  return data;
}

async function performPageTask(task, successText, errorText) {
  pageLoading.value = true;
  try {
    const result = await task();
    if (successText) {
      message.success(successText);
    }
    return result;
  } catch (error) {
    message.error(error.message || errorText || "请求失败");
    throw error;
  } finally {
    pageLoading.value = false;
  }
}

function markViewSuccess(view) {
  viewLoaded[view] = true;
  viewState[view].error = "";
  viewState[view].syncedAt = Date.now();
}

function markViewError(view, error) {
  viewState[view].error = error.message || "同步失败";
}

async function runViewLoad(view) {
  try {
    if (view === "dashboard") await fetchDashboard();
    if (view === "users") await fetchUsers();
    if (view === "whitelist") await fetchWhitelist();
    if (view === "bans") await fetchBans();
    if (view === "settings") await fetchSettings();
    if (view === "logs") await fetchLogs();
    markViewSuccess(view);
  } catch (error) {
    markViewError(view, error);
    throw error;
  }
}

async function fetchDashboard() {
  const data = await request("/guard/api/dashboard");
  Object.assign(dashboard, clone(data, dashboard));
}

async function fetchUsers() {
  const query = new URLSearchParams({
    page: String(userQuery.page),
    size: String(userQuery.size),
    search: userQuery.search || ""
  });
  const data = await request(`/guard/api/users?${query.toString()}`);
  users.value = data.items || [];
  userQuery.total = data.total || users.value.length;
}

async function fetchWhitelist() {
  const data = await request("/guard/api/whitelist");
  whitelist.value = data.items || [];
}

async function fetchBans() {
  const [active, history] = await Promise.all([request("/guard/api/bans?status=active"), request("/guard/api/bans?status=all")]);
  activeBans.value = active.items || [];
  banHistory.value = history.items || [];
}

async function fetchSettings() {
  const data = await request("/guard/api/settings");
  const payload = data.data || {};
  Object.assign(settingsModel, clone(payload, {}));
  settingsText.allowed_ua = formatMaybeArray(payload.allowed_ua);
  settingsText.discord_oauth_scopes = formatMaybeArray(payload.discord_oauth_scopes);
  settingsText.discord_access_policy = formatMaybeJson(payload.discord_access_policy);
}

async function fetchLogs() {
  await Promise.all([fetchBanLogs(), fetchCheckinLogs(), fetchStatsLogs()]);
}

async function fetchBanLogs() {
  const data = await request("/guard/api/logs/bans?page=1&size=50");
  banLogs.value = data.items || [];
}

async function fetchCheckinLogs() {
  const params = new URLSearchParams({ page: "1", size: "50" });
  if (checkinFilter.value) {
    params.set("user_id", checkinFilter.value);
  }
  const data = await request(`/guard/api/logs/checkins?${params.toString()}`);
  checkinLogs.value = data.items || [];
}

async function fetchStatsLogs() {
  const data = await request("/guard/api/logs/stats?days=30");
  statsLogs.value = data.items || [];
}

async function submitLogin() {
  if (!login.password) {
    login.notice = "请输入管理密码。";
    return;
  }
  authLoading.value = true;
  try {
    const data = await request("/guard/api/auth/login", {
      method: "POST",
      body: { password: login.password }
    });
    auth.token = data.token;
    localStorage.setItem("guard_token", auth.token);
    login.notice = "登录成功，正在同步控制台数据。";
    await refreshAll();
  } catch (error) {
    login.notice = error.message || "登录失败";
    message.error(login.notice);
  } finally {
    authLoading.value = false;
  }
}

async function logout(silent = false) {
  if (auth.token) {
    try {
      await fetch("/guard/api/auth/logout", {
        method: "POST",
        headers: { Authorization: `Bearer ${auth.token}` }
      });
    } catch {
      // 忽略退出时的网络波动
    }
  }
  auth.token = "";
  login.password = "";
  localStorage.removeItem("guard_token");
  Object.keys(viewLoaded).forEach((key) => {
    viewLoaded[key] = false;
    viewState[key].error = "";
    viewState[key].syncedAt = 0;
  });
  users.value = [];
  whitelist.value = [];
  activeBans.value = [];
  banHistory.value = [];
  banLogs.value = [];
  checkinLogs.value = [];
  statsLogs.value = [];
  if (!silent) {
    message.success("已退出控制台");
  }
}

function openView(view) {
  activeView.value = view;
}

async function refreshAll() {
  await performPageTask(async () => {
    const views = ["dashboard", "users", "whitelist", "bans", "settings", "logs"];
    const results = await Promise.allSettled(views.map((view) => runViewLoad(view)));
    const failed = results.filter((item) => item.status === "rejected");
    if (failed.length) {
      throw new Error(`有 ${failed.length} 个视图同步失败，请查看对应错误提示`);
    }
  }, "已完成全量同步");
}

async function refreshCurrentView() {
  await performPageTask(async () => {
    await runViewLoad(activeView.value);
  }, "当前视图已刷新");
}

async function ensureView(view) {
  if (!auth.token || viewLoaded[view]) return;
  await runViewLoad(view);
}

async function applyUserSearch() {
  userQuery.page = 1;
  userQuery.search = userSearchDraft.value.trim();
  await performPageTask(async () => {
    await runViewLoad("users");
  }, "用户列表已更新");
}

async function handleUserPageChange(page) {
  userQuery.page = page;
  await performPageTask(async () => {
    await runViewLoad("users");
  });
}

async function handleUserPageSizeChange(size) {
  userQuery.size = size;
  userQuery.page = 1;
  await performPageTask(async () => {
    await runViewLoad("users");
  });
}

async function resetUserSearch() {
  userSearchDraft.value = "";
  userQuery.search = "";
  userQuery.page = 1;
  await performPageTask(async () => {
    await runViewLoad("users");
  }, "已清空搜索条件");
}

async function refreshLogsOnly() {
  await performPageTask(async () => {
    await runViewLoad("logs");
  }, "日志已刷新");
}

async function loadCheckinLogsAction() {
  await performPageTask(async () => {
    await fetchCheckinLogs();
    markViewSuccess("logs");
  }, "签到日志已刷新");
}

async function toggleWhitelist(item) {
  const method = item.is_whitelist ? "DELETE" : "POST";
  await performPageTask(async () => {
    await request(`/guard/api/whitelist/${item.newapi_user_id}`, { method });
    await Promise.all([runViewLoad("users"), runViewLoad("whitelist"), runViewLoad("dashboard")]);
  }, item.is_whitelist ? "已移出白名单" : "已加入白名单");
}

function resetCreateUserModal() {
  createUserModal.form.mode = "password";
  createUserModal.form.username = "";
  createUserModal.form.password = "";
  createUserModal.form.discord_id = "";
  createUserModal.form.discord_name = "";
  createUserModal.form.initial_quota = 0;
  createUserModal.form.is_whitelist = false;
}

function openCreateUserModal() {
  resetCreateUserModal();
  createUserModal.show = true;
}

async function submitCreateUser() {
  createUserModal.loading = true;
  try {
    await request("/guard/api/users", {
      method: "POST",
      body: clone(createUserModal.form, {})
    });
    createUserModal.show = false;
    message.success("用户已创建");
    await Promise.all([runViewLoad("users"), runViewLoad("whitelist"), runViewLoad("dashboard")]);
  } catch (error) {
    message.error(error.message || "创建失败");
  } finally {
    createUserModal.loading = false;
  }
}

function resetBanModal() {
  banModal.form.user_ref = "";
  banModal.form.newapi_user_id = null;
  banModal.form.discord_id = "";
  banModal.form.reason = "";
  banModal.form.duration = "permanent";
}

function openBanModal(item = null) {
  resetBanModal();
  if (item) {
    banModal.form.newapi_user_id = item.newapi_user_id || null;
    banModal.form.user_ref = item.discord_id || String(item.newapi_user_id || "");
    banModal.form.discord_id = item.discord_id || "";
    banModal.form.reason = item.reason && item.reason !== "无上下文（可能直接在 New API 后台封禁）" ? item.reason : "";
  }
  banModal.show = true;
}

async function submitBan() {
  if (!banModal.form.reason) {
    message.warning("请先填写封禁原因");
    return;
  }
  banModal.loading = true;
  try {
    await request("/guard/api/bans", {
      method: "POST",
      body: {
        user_ref: banModal.form.user_ref,
        newapi_user_id: banModal.form.newapi_user_id,
        discord_id: banModal.form.discord_id,
        reason: banModal.form.reason,
        duration: banModal.form.duration
      }
    });
    banModal.show = false;
    message.success("封禁操作已提交");
    await Promise.all([runViewLoad("bans"), runViewLoad("dashboard"), runViewLoad("users"), fetchBanLogs().then(() => markViewSuccess("logs"))]);
  } catch (error) {
    message.error(error.message || "封禁失败");
  } finally {
    banModal.loading = false;
  }
}

async function quickUnban(item) {
  const payload = {
    user_ref: item.discord_id || String(item.newapi_user_id || ""),
    newapi_user_id: item.newapi_user_id,
    discord_id: item.discord_id || ""
  };

  dialog.warning({
    title: "确认解除封禁",
    content: `将对 ${item.display_name || item.username || item.newapi_user_id} 执行解除封禁操作。`,
    positiveText: "确认解除",
    negativeText: "取消",
    onPositiveClick: async () => {
      try {
        await request("/guard/api/bans/unban", {
          method: "POST",
          body: payload
        });
        message.success("已解除封禁");
        await Promise.all([runViewLoad("bans"), runViewLoad("dashboard"), runViewLoad("users"), fetchBanLogs().then(() => markViewSuccess("logs"))]);
      } catch (error) {
        message.error(error.message || "解除封禁失败");
      }
    }
  });
}

async function saveSettings() {
  try {
    const payload = clone(settingsModel, {});
    payload.allowed_ua = settingsText.allowed_ua
      .split(/\n+/)
      .map((item) => item.trim())
      .filter(Boolean);
    payload.discord_oauth_scopes = settingsText.discord_oauth_scopes
      .split(/\n+/)
      .map((item) => item.trim())
      .filter(Boolean);
    payload.discord_access_policy = JSON.parse(settingsText.discord_access_policy || "{}");

    await performPageTask(async () => {
      await request("/guard/api/settings", { method: "PUT", body: payload });
      await Promise.all([runViewLoad("settings"), runViewLoad("dashboard")]);
    }, "设置已保存");
  } catch (error) {
    message.error(error.message || "设置保存失败");
  }
}

function resetSettingsDraft() {
  settingsText.allowed_ua = formatMaybeArray(settingsModel.allowed_ua);
  settingsText.discord_oauth_scopes = formatMaybeArray(settingsModel.discord_oauth_scopes);
  settingsText.discord_access_policy = formatMaybeJson(settingsModel.discord_access_policy);
  message.info("已恢复为当前已加载配置");
}

function switchBanMode(mode) {
  banMode.value = mode;
}

function trendWidth(value) {
  const max = Math.max(...statsLogs.value.map((item) => Number(item.total_requests || 0)), 1);
  return `${Math.max(8, Math.round((Number(value || 0) / max) * 100))}%`;
}

watch(activeView, (view) => {
  ensureView(view).catch((error) => {
    message.error(error.message || "视图加载失败");
  });
});

onMounted(async () => {
  userSearchDraft.value = userQuery.search;
  if (auth.token) {
    try {
      await refreshAll();
    } catch {
      await logout(true);
    }
  }
});
</script>
