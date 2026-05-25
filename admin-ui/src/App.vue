<template>
  <n-config-provider :theme="naiveTheme" :theme-overrides="themeOverrides">
    <template v-if="!auth.token">
      <div class="login-shell">
        <div class="login-panel">
          <section class="frame login-card">
            <div>
              <div class="panel-label">登录</div>
              <h2 class="panel-title">进入 Guard 控制台</h2>
            </div>

            <form class="login-form" @submit.prevent="submitLogin">
              <label class="login-field" for="login-password">
                <span class="login-field-label">管理员密码</span>
                <input
                  id="login-password"
                  v-model="login.password"
                  class="login-password-field"
                  type="password"
                  placeholder="请输入管理员密码"
                  autocomplete="current-password"
                />
              </label>
            </form>

            <p v-if="login.notice" class="login-notice">{{ login.notice }}</p>

            <div class="auth-row">
              <n-button type="primary" size="large" :loading="authLoading" @click="submitLogin">进入 Guard 控制台</n-button>
            </div>
          </section>
        </div>
      </div>
    </template>

    <template v-else>
      <div class="tool-shell">
        <div class="tool-layout" :class="{ collapsed: sidebarCollapsed }">
          <aside class="nav-frame" :class="{ collapsed: sidebarCollapsed }">
            <button class="nav-collapse" type="button" :aria-label="sidebarCollapsed ? '展开侧边栏' : '收起侧边栏'" @click="sidebarCollapsed = !sidebarCollapsed">
              {{ sidebarCollapsed ? ">" : "<" }}
            </button>

            <div class="brand-stack" :class="{ collapsed: sidebarCollapsed }">
              <div v-if="!sidebarCollapsed" class="hero-eyebrow">公益站守卫层</div>
              <div class="brand-title">Guard</div>
              <div v-if="!sidebarCollapsed" class="brand-chip-row">
                <div class="brand-chip"><strong>{{ dashboard.active_bans || 0 }}</strong> 当前封禁</div>
                <div class="brand-chip"><strong>{{ dashboard.whitelist_count || 0 }}</strong> 白名单</div>
              </div>
            </div>

            <nav class="nav-list">
              <button
                v-for="item in navItems"
                :key="item.key"
                class="nav-button"
                :class="{ active: activeView === item.key, collapsed: sidebarCollapsed }"
                @click="openView(item.key)"
              >
                <div>
                  <strong>{{ item.label }}</strong>
                  <span v-if="!sidebarCollapsed">{{ item.hint }}</span>
                </div>
                <div v-if="!sidebarCollapsed" class="nav-count">{{ navCount(item.key) }}</div>
              </button>
            </nav>

            <div class="theme-switch" :class="{ collapsed: sidebarCollapsed }">
              <div class="theme-segment" :aria-label="`主题模式，当前${currentThemeLabel}`" role="group">
                <button
                  v-for="option in themeOptions"
                  :key="option.value"
                  class="theme-option theme-option-icon"
                  :class="{ active: themeMode === option.value }"
                  type="button"
                  :aria-label="`切换到${option.label}`"
                  :aria-pressed="themeMode === option.value"
                  :title="option.label"
                  @click="themeMode = option.value"
                >
                  <svg v-if="option.value === 'system'" viewBox="0 0 24 24" aria-hidden="true">
                    <rect x="4" y="5" width="16" height="11" rx="2.5" />
                    <path d="M9 19h6" />
                    <path d="M12 16v3" />
                  </svg>
                  <svg v-else-if="option.value === 'light'" viewBox="0 0 24 24" aria-hidden="true">
                    <circle cx="12" cy="12" r="4.2" />
                    <path d="M12 2.5v3" />
                    <path d="M12 18.5v3" />
                    <path d="M21.5 12h-3" />
                    <path d="M5.5 12h-3" />
                    <path d="M18.7 5.3l-2.1 2.1" />
                    <path d="M7.4 16.6l-2.1 2.1" />
                    <path d="M18.7 18.7l-2.1-2.1" />
                    <path d="M7.4 7.4L5.3 5.3" />
                  </svg>
                  <svg v-else viewBox="0 0 24 24" aria-hidden="true">
                    <path d="M19 14.8A7.2 7.2 0 0 1 9.2 5a7.3 7.3 0 1 0 9.8 9.8Z" />
                  </svg>
                  <span class="sr-only">{{ option.label }}</span>
                </button>
              </div>
            </div>
          </aside>

          <main class="main-frame">
            <header class="main-head">
              <div>
                <div class="section-kicker">{{ sectionMeta.kicker }}</div>
                <h2 class="section-title">{{ sectionMeta.title }}</h2>
              </div>
              <div class="main-actions">
                <n-button secondary @click="refreshCurrentView">刷新当前视图</n-button>
                <n-button tertiary @click="refreshAll">全量同步</n-button>
                <n-button quaternary @click="logout">退出</n-button>
              </div>
            </header>

            <n-spin :show="pageLoading">
              <div class="content-scroll">
                <section v-if="currentViewError" class="section-stack section-top">
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
                    </article>
                    <article class="stat-card">
                      <div class="capsule-label">当前封禁</div>
                      <div class="stat-value">{{ formatNumber(dashboard.active_bans) }}</div>
                    </article>
                    <article class="stat-card">
                      <div class="capsule-label">白名单</div>
                      <div class="stat-value">{{ formatNumber(dashboard.whitelist_count) }}</div>
                    </article>
                    <article class="stat-card">
                      <div class="capsule-label">新增封禁</div>
                      <div class="stat-value">{{ formatNumber(dashboard.today?.new_bans) }}</div>
                    </article>
                  </div>

                  <div class="panel-grid dashboard-insight-grid">
                    <article class="overview-block status-summary-card">
                      <h3 class="block-title">当前状态</h3>
                      <div class="status-summary-row">
                        <span class="capsule-label">当前状态</span>
                        <strong class="status-summary-value">{{ currentViewHealth.label }}</strong>
                      </div>
                      <div class="status-summary-row">
                        <span class="capsule-label">透明代理目标</span>
                        <strong class="status-summary-value mono">{{ settingsModel.newapi_base_url || "未配置" }}</strong>
                      </div>
                      <div class="status-summary-row">
                        <span class="capsule-label">用户级 RPM</span>
                        <strong class="status-summary-value">{{ settingsModel.rpm_limit || "—" }} / 分钟</strong>
                      </div>
                      <div class="status-summary-row">
                        <span class="capsule-label">最近同步</span>
                        <strong class="status-summary-value">{{ currentViewSyncedAt }}</strong>
                      </div>
                    </article>

                    <article class="overview-block">
                      <h3 class="block-title">今日风险面板</h3>
                      <div class="signal-line">
                        <div class="signal-name">UA 拦截次数</div>
                        <div class="signal-value">{{ formatNumber(dashboard.today?.blocked_ua) }}</div>
                      </div>
                      <div class="signal-line">
                        <div class="signal-name">RPM 拦截次数</div>
                        <div class="signal-value">{{ formatNumber(dashboard.today?.blocked_rpm) }}</div>
                      </div>
                      <div class="signal-line">
                        <div class="signal-name">签到完成次数</div>
                        <div class="signal-value">{{ formatNumber(dashboard.today?.checkins) }}</div>
                      </div>
                      <div class="signal-line">
                        <div class="signal-name">新增用户</div>
                        <div class="signal-value">{{ formatNumber(dashboard.today?.new_users) }}</div>
                      </div>
                    </article>
                  </div>

                  <div class="panel-grid dashboard-preview-grid">
                    <article class="overview-block dashboard-preview-card">
                      <div class="panel-heading">
                        <div>
                          <h3 class="block-title">当前封禁预览</h3>
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

                    <article class="overview-block dashboard-preview-card">
                      <div class="panel-heading">
                        <div>
                          <h3 class="block-title">统计趋势预览</h3>
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
                    <input
                      v-model="userSearchDraft"
                      class="app-input filter-input"
                      type="text"
                      placeholder="搜索 newapi id、discord id、discord 名称或用户名"
                      @keyup.enter="applyUserSearch"
                    />
                    <n-button type="primary" @click="applyUserSearch">搜索</n-button>
                    <n-button tertiary @click="resetUserSearch">清空</n-button>
                    <n-button secondary @click="openCreateUserModal">创建用户</n-button>
                  </div>

                  <div class="toolbar-split soft-card">
                    <div class="table-meta">
                      {{ userQuery.search ? "关键字：" + userQuery.search + " · " : "" }}共 {{ formatNumber(userQuery.total || users.length) }} 条 · 第
                      {{ userQuery.page }} / {{ userPageCount }} 页
                    </div>
                    <div class="table-actions">
                      <select class="app-select page-size-select" :value="userQuery.size" @change="handleUserPageSizeChange(Number($event.target.value))">
                        <option v-for="option in userPageSizeOptions" :key="'user-size-' + option.value" :value="option.value">{{ option.label }}</option>
                      </select>
                      <n-pagination :page="userQuery.page" :page-count="userPageCount" simple @update:page="handleUserPageChange" />
                    </div>
                  </div>

                  <article class="overview-block">
                    <div class="panel-heading">
                      <div>
                        <h3 class="block-title">用户列表</h3>
                      </div>
                      <n-tag size="large" :bordered="false">共 {{ formatNumber(userQuery.total || users.length) }} 条 · 本页 {{ users.length }} 条</n-tag>
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
                                <div class="cell-title">{{ formatQuotaUSD(item.quota) }}</div>
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
                                <div class="cell-sub">白名单</div>
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
                    <input v-model="banFilter" class="app-input ban-filter" type="text" placeholder="按用户、Discord、原因或 UA 过滤当前列表" />
                  </div>

                  <div class="signal-grid">
                    <article class="signal-card">
                      <div class="capsule-label">当前封禁</div>
                      <div class="signal-value">{{ formatNumber(activeBans.length) }}</div>
                    </article>
                    <article class="signal-card">
                      <div class="capsule-label">含 Guard 上下文</div>
                      <div class="signal-value">{{ formatNumber(activeBansWithContext) }}</div>
                    </article>
                    <article class="signal-card">
                      <div class="capsule-label">历史记录</div>
                      <div class="signal-value">{{ formatNumber(banHistory.length) }}</div>
                    </article>
                    <article class="signal-card">
                      <div class="capsule-label">本地过滤</div>
                      <div class="signal-value">{{ banFilter || "未启用" }}</div>
                    </article>
                  </div>

                  <article class="overview-block">
                    <div class="panel-heading">
                      <div>
                        <h3 class="block-title">{{ banMode === "active" ? "当前封禁列表" : "封禁历史记录" }}</h3>
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
                  <div class="settings-stack-page">
                    <article class="overview-block settings-stack">
                      <div>
                        <h3 class="block-title">代理与运维设置</h3>
                      </div>
                      <div class="compact-grid">
                        <label class="field">
                          <span class="field-label">New API 内网地址</span>
                          <input v-model="settingsModel.newapi_base_url" class="app-input" type="text" placeholder="http://new-api:3000" />
                        </label>
                        <label class="field">
                          <span class="field-label">New API 管理员令牌</span>
                          <input v-model="settingsModel.newapi_admin_token" class="app-input" type="text" placeholder="用于用户与封禁操作" />
                        </label>
                        <label class="field">
                          <span class="field-label">控制台公开地址</span>
                          <input v-model="settingsModel.public_base_url" class="app-input" type="text" placeholder="可留空自动推断" />
                        </label>
                        <label class="field">
                          <span class="field-label">管理员密码</span>
                          <input v-model="settingsModel.admin_password" class="app-input" type="text" placeholder="控制台登录密码" />
                        </label>
                        <label class="field">
                          <span class="field-label">用户级 RPM 限制</span>
                          <input v-model.number="settingsModel.rpm_limit" class="app-input" type="number" min="1" />
                        </label>
                        <label class="field">
                          <span class="field-label">UA 违规封禁阈值</span>
                          <input v-model.number="settingsModel.ua_ban_strikes" class="app-input" type="number" min="1" />
                        </label>
                        <label class="field">
                          <span class="field-label">签到补发额度</span>
                          <input v-model.number="settingsModel.checkin_quota" class="app-input" type="number" min="0" />
                        </label>
                        <label class="field">
                          <span class="field-label">签到余额阈值</span>
                          <input v-model.number="settingsModel.checkin_threshold" class="app-input" type="number" min="0" />
                        </label>
                      </div>
                      <div class="field field-full">
                        <span class="field-label field-label-with-help">
                          <span>允许的 UA 前缀</span>
                          <button class="help-icon" type="button" aria-label="查看允许的 UA 前缀说明" @click="openHelp('allowed_ua')">?</button>
                        </span>
                        <div class="settings-visual-stack">
                          <p class="field-caption">通过条目维护允许前缀，保存时仍会写成字符串数组。</p>

                          <div v-if="allowedUADraft.items.length" class="list-editor-stack">
                            <div v-for="(item, index) in allowedUADraft.items" :key="'allowed-ua-' + index" class="list-editor-row soft-card">
                              <template v-if="allowedUADraft.editIndex === index">
                                <input
                                  v-model="allowedUADraft.editValue"
                                  class="app-input list-editor-input mono"
                                  type="text"
                                  placeholder="例如 FLClash/"
                                  @keyup.enter="saveAllowedUAEdit"
                                />
                                <div class="action-row">
                                  <n-button size="small" type="primary" @click="saveAllowedUAEdit">保存</n-button>
                                  <n-button size="small" tertiary @click="cancelAllowedUAEdit">取消</n-button>
                                </div>
                              </template>
                              <template v-else>
                                <div class="list-editor-content">
                                  <div class="cell-title mono">{{ item }}</div>
                                  <div class="field-caption">前缀 {{ index + 1 }}</div>
                                </div>
                                <div class="action-row">
                                  <n-button size="small" secondary @click="startAllowedUAEdit(index)">编辑</n-button>
                                  <n-button size="small" type="warning" ghost @click="removeAllowedUAItem(index)">删除</n-button>
                                </div>
                              </template>
                            </div>
                          </div>
                          <div v-else class="section-note">当前没有配置允许的 UA 前缀。留空时表示不启用 UA 限制。</div>

                          <div class="list-editor-create">
                            <input
                              v-model="allowedUADraft.pending"
                              class="app-input list-editor-input"
                              type="text"
                              placeholder="新增一个允许前缀，例如 FLClash/"
                              @keyup.enter="addAllowedUAItem"
                            />
                            <n-button secondary @click="addAllowedUAItem">新增前缀</n-button>
                          </div>
                        </div>
                      </div>
                    </article>

                    <article class="overview-block settings-stack">
                      <div>
                        <h3 class="block-title">OAuth 与 Discord 准入</h3>
                      </div>
                      <div class="compact-grid">
                        <label class="field">
                          <span class="field-label">OAuth Client ID</span>
                          <input v-model="settingsModel.oauth_client_id" class="app-input" type="text" />
                        </label>
                        <label class="field">
                          <span class="field-label">OAuth Client Secret</span>
                          <input v-model="settingsModel.oauth_client_secret" class="app-input" type="text" autocomplete="off" />
                        </label>
                        <label class="field">
                          <span class="field-label">Provider Slug</span>
                          <input v-model="settingsModel.oauth_provider_slug" class="app-input" type="text" />
                        </label>
                        <label class="field">
                          <span class="field-label">Discord Client ID</span>
                          <input v-model="settingsModel.discord_client_id" class="app-input" type="text" />
                        </label>
                        <label class="field">
                          <span class="field-label">Discord Client Secret</span>
                          <input v-model="settingsModel.discord_client_secret" class="app-input" type="text" autocomplete="off" />
                        </label>
                        <label class="field">
                          <span class="field-label">Discord Guild ID</span>
                          <input v-model="settingsModel.discord_guild_id" class="app-input" type="text" />
                        </label>
                        <label class="field">
                          <span class="field-label">State TTL（秒）</span>
                          <input v-model.number="settingsModel.oauth_state_ttl_seconds" class="app-input" type="number" min="60" />
                        </label>
                        <label class="field">
                          <span class="field-label">Code TTL（秒）</span>
                          <input v-model.number="settingsModel.oauth_code_ttl_seconds" class="app-input" type="number" min="60" />
                        </label>
                        <label class="field">
                          <span class="field-label">Token TTL（秒）</span>
                          <input v-model.number="settingsModel.oauth_token_ttl_seconds" class="app-input" type="number" min="60" />
                        </label>
                      </div>
                      <div class="field field-full">
                        <span class="field-label">Discord OAuth Scopes</span>
                        <textarea v-model="settingsText.discord_oauth_scopes" class="app-textarea" rows="4" placeholder="每行一个 scope，例如 identify"></textarea>
                      </div>
                      <div class="field field-full">
                        <span class="field-label field-label-with-help">
                          <span>Discord 准入规则</span>
                          <button class="help-icon" type="button" aria-label="查看 Discord 准入规则说明" @click="openHelp('discord_access_policy')">?</button>
                        </span>
                        <div class="settings-visual-stack">
                          <p class="field-caption">在界面里维护条件和规则组，保存时仍会写回后端当前使用的 JSON 结构。</p>
                          <PolicyGroupEditor :group="discordAccessPolicyDraft" root />
                        </div>
                      </div>
                    </article>
                  </div>

                  <article class="overview-block settings-actions">
                    <div class="action-row">
                      <n-button secondary @click="resetSettingsDraft">重置草稿</n-button>
                      <n-button type="primary" @click="saveSettings">保存设置</n-button>
                    </div>
                  </article>
                </section>

                <section v-else-if="activeView === 'logs'" class="section-stack">
                  <article class="overview-block">
                    <div class="panel-heading">
                      <div>
                        <h3 class="block-title">运维日志</h3>
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
                          <input v-model="checkinFilter" class="app-input filter-input" type="text" placeholder="按 newapi 用户 ID 过滤" @keyup.enter="loadCheckinLogsAction" />
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
            <label class="field">
              <span class="field-label">用户名</span>
              <input v-model="createUserModal.form.username" class="app-input" type="text" placeholder="例如 tom" />
            </label>
            <label class="field">
              <span class="field-label">密码</span>
              <input v-model="createUserModal.form.password" class="app-input" type="password" />
            </label>
          </div>
          <div class="compact-grid" v-else>
            <label class="field">
              <span class="field-label">Discord ID</span>
              <input v-model="createUserModal.form.discord_id" class="app-input" type="text" placeholder="例如 298374928374" />
            </label>
            <label class="field">
              <span class="field-label">Discord 名称</span>
              <input v-model="createUserModal.form.discord_name" class="app-input" type="text" placeholder="例如 Tom#1234" />
            </label>
          </div>
          <div class="compact-grid">
            <label class="field">
              <span class="field-label">初始额度</span>
              <input v-model.number="createUserModal.form.initial_quota" class="app-input" type="number" min="0" />
            </label>
            <label class="field checkbox-field">
              <span class="field-label">是否加入白名单</span>
              <span class="checkbox-row">
                <input v-model="createUserModal.form.is_whitelist" class="app-checkbox" type="checkbox" />
                <span class="checkbox-text">{{ createUserModal.form.is_whitelist ? "是" : "否" }}</span>
              </span>
            </label>
          </div>
          <div class="action-row">
            <n-button secondary @click="createUserModal.show = false">取消</n-button>
            <n-button type="primary" :loading="createUserModal.loading" @click="submitCreateUser">确认创建</n-button>
          </div>
        </div>
      </n-modal>

      <n-modal v-model:show="banModal.show" preset="card" style="width:min(760px, 92vw)" title="处理封禁">
        <div class="settings-stack">
          <div class="compact-grid">
            <label class="field">
              <span class="field-label">统一用户标识</span>
              <input v-model="banModal.form.user_ref" class="app-input" type="text" placeholder="例如 123 或 298374928374" />
            </label>
            <label class="field">
              <span class="field-label">Discord ID（可选）</span>
              <input v-model="banModal.form.discord_id" class="app-input" type="text" placeholder="优先用于 Discord 映射查找" />
            </label>
            <label class="field">
              <span class="field-label">newapi 用户 ID（可选）</span>
              <input v-model.number="banModal.form.newapi_user_id" class="app-input" type="number" min="1" />
            </label>
            <label class="field">
              <span class="field-label">封禁时长</span>
              <select v-model="banModal.form.duration" class="app-select">
                <option v-for="option in banDurationOptions" :key="'ban-duration-' + option.value" :value="option.value">{{ option.label }}</option>
              </select>
            </label>
          </div>
          <label class="field field-full">
            <span class="field-label">封禁原因</span>
            <textarea v-model="banModal.form.reason" class="app-textarea" rows="5" placeholder="记录操作原因"></textarea>
          </label>
          <div class="action-row">
            <n-button secondary @click="banModal.show = false">取消</n-button>
            <n-button type="warning" :loading="banModal.loading" @click="submitBan">确认封禁</n-button>
          </div>
        </div>
      </n-modal>

      <n-modal v-model:show="helpModal.show" preset="card" style="width:min(720px, 92vw)" :title="helpContent.title">
        <div class="help-stack">
          <section class="help-block">
            <div class="help-label">解释</div>
            <p class="help-text">{{ helpContent.explain }}</p>
          </section>
          <section class="help-block">
            <div class="help-label">简单写法</div>
            <p class="help-text">{{ helpContent.howto }}</p>
          </section>
          <section class="help-block">
            <div class="help-label">示例</div>
            <pre class="help-code">{{ helpContent.example }}</pre>
          </section>
        </div>
      </n-modal>
    </template>
  </n-config-provider>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { createDiscreteApi, darkTheme } from "naive-ui";
import PolicyGroupEditor from "./components/PolicyGroupEditor.vue";

const themeOptions = [
  { value: "system", label: "自动" },
  { value: "light", label: "日间" },
  { value: "dark", label: "夜间" }
];

const storedThemeMode = localStorage.getItem("guard_theme_mode");
const themeMode = ref(themeOptions.some((option) => option.value === storedThemeMode) ? storedThemeMode : "system");
const systemTheme = ref("dark");
const resolvedTheme = computed(() => (themeMode.value === "system" ? systemTheme.value : themeMode.value));
const currentThemeLabel = computed(() => {
  const currentThemeOption = themeOptions.find((option) => option.value === themeMode.value);
  return currentThemeOption ? currentThemeOption.label : themeOptions[0].label;
});
const naiveTheme = computed(() => (resolvedTheme.value === "dark" ? darkTheme : null));

const themeOverrides = computed(() => {
  const isDark = resolvedTheme.value === "dark";
  return {
    common: {
      primaryColor: "oklch(0.78 0.124 174)",
      primaryColorHover: "oklch(0.83 0.11 174)",
      primaryColorPressed: "oklch(0.72 0.11 174)",
      infoColor: "oklch(0.78 0.124 174)",
      successColor: "oklch(0.76 0.12 160)",
      warningColor: "oklch(0.8 0.12 78)",
      errorColor: "oklch(0.7 0.16 27)",
      bodyColor: isDark ? "oklch(0.19 0.02 220)" : "oklch(0.985 0.008 210)",
      cardColor: isDark ? "oklch(0.235 0.022 220)" : "oklch(0.998 0.006 210)",
      modalColor: isDark ? "oklch(0.235 0.022 220)" : "oklch(0.998 0.006 210)",
      popoverColor: isDark ? "oklch(0.235 0.022 220)" : "oklch(0.998 0.006 210)",
      borderColor: isDark ? "rgba(255,255,255,0.08)" : "rgba(27,52,67,0.12)",
      textColorBase: isDark ? "oklch(0.92 0.01 220)" : "oklch(0.25 0.018 220)",
      textColor2: isDark ? "oklch(0.75 0.014 220)" : "oklch(0.48 0.016 220)",
      fontFamily: "\"Noto Sans SC\", \"Segoe UI\", sans-serif",
      fontFamilyMono: "\"Chakra Petch\", sans-serif"
    },
    Button: {
      borderRadiusMedium: "14px",
      borderRadiusSmall: "12px"
    },
    Input: {
      color: isDark ? "rgba(255,255,255,0.02)" : "rgba(255,255,255,0.78)",
      borderRadius: "14px"
    }
  };
});

const { message, dialog } = createDiscreteApi(["message", "dialog"], {
  configProviderProps: {
    theme: naiveTheme.value,
    themeOverrides: themeOverrides.value
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
const sidebarCollapsed = ref(false);
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
  discord_oauth_scopes: ""
});
const allowedUADraft = reactive({
  items: [],
  pending: "",
  editIndex: -1,
  editValue: ""
});
const discordAccessPolicyDraft = ref(createPolicyGroup());

const helpTopics = {
  allowed_ua: {
    title: "允许的 UA 前缀",
    explain:
      "后端会把这里当成字符串数组处理，逐条用前缀匹配请求头里的 User-Agent。这里留空时，后端会直接放行全部 UA，不启用 UA 限制。",
    howto:
      "在界面里逐条新增、编辑或删除前缀即可，不需要自己写数组格式。保存时前端会自动整理成后端需要的字符串数组。",
    example: `条目示例
FLClash/
Shadowrocket/
ClashMeta/
OpenClash/`
  },
  discord_access_policy: {
    title: "Discord 准入规则",
    explain:
      "后端会把这里按 JSON 对象解析，并递归执行 logic / conditions / groups。当前代码只支持两种条件：field=\"guild_id\" 配 op=\"eq\"，以及 field=\"roles\" 配 op=\"contains\"。其中 guild_id 对应当前 Discord 服务器 ID，roles 对应 Discord 返回的成员角色 ID 列表，不认角色名称。",
    howto:
      "直接在界面里选择 AND / OR、增加条件或嵌套规则组即可。想要求“必须在某个服务器，并且拥有 A 或 B 角色之一”，就把根规则设成 AND，再新增一个 OR 子规则组来放多个角色条件。空规则会被后端视为全部放行。",
    example: `示例结构
根规则：AND
条件 1：服务器 ID = 123456789012345678
子规则组：OR
- 角色 ID = 987654321000000001
- 角色 ID = 987654321000000002`
  }
};

const helpModal = reactive({
  show: false,
  key: "allowed_ua"
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
const helpContent = computed(() => helpTopics[helpModal.key] || helpTopics.allowed_ua);
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

let colorSchemeMediaQuery;
let removeColorSchemeListener;

function clone(value, fallback) {
  return JSON.parse(JSON.stringify(value ?? fallback));
}

function applyResolvedTheme(theme) {
  if (typeof document === "undefined") return;
  document.documentElement.dataset.theme = theme;
  document.documentElement.style.colorScheme = theme;
}

function openHelp(key) {
  helpModal.key = key;
  helpModal.show = true;
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

function formatQuotaUSD(value) {
  const amount = Number(value || 0) / 500000;
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: Number.isInteger(amount) ? 0 : 2,
    maximumFractionDigits: 2
  }).format(amount);
}

function formatMaybeArray(value) {
  return Array.isArray(value) ? value.join("\n") : "";
}

function createLocalId() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return `guard-${Math.random().toString(16).slice(2)}-${Date.now()}`;
}

function createPolicyCondition(field = "guild_id", value = "") {
  const normalizedField = field === "roles" ? "roles" : "guild_id";
  return {
    id: createLocalId(),
    field: normalizedField,
    op: normalizedField === "roles" ? "contains" : "eq",
    value: String(value ?? "")
  };
}

function normalizePolicyCondition(condition) {
  if (!condition || typeof condition !== "object") {
    return null;
  }
  return createPolicyCondition(condition.field, condition.value);
}

function createPolicyGroup(group = {}) {
  return {
    id: createLocalId(),
    logic: String(group.logic || "").toLowerCase() === "or" ? "or" : "and",
    conditions: Array.isArray(group.conditions) ? group.conditions.map(normalizePolicyCondition).filter(Boolean) : [],
    groups: Array.isArray(group.groups) ? group.groups.map((item) => createPolicyGroup(item)) : []
  };
}

function parsePolicyValue(value) {
  if (!value) {
    return createPolicyGroup();
  }
  if (typeof value === "string") {
    try {
      return createPolicyGroup(JSON.parse(value));
    } catch {
      return createPolicyGroup();
    }
  }
  if (typeof value === "object") {
    return createPolicyGroup(value);
  }
  return createPolicyGroup();
}

function serializePolicyGroup(group) {
  const conditions = (group.conditions || [])
    .map((condition) => ({
      field: condition.field === "roles" ? "roles" : "guild_id",
      op: condition.field === "roles" ? "contains" : "eq",
      value: String(condition.value || "").trim()
    }))
    .filter((condition) => condition.value);

  const groups = (group.groups || [])
    .map((item) => serializePolicyGroup(item))
    .filter((item) => item.conditions.length || item.groups.length);

  return {
    logic: group.logic === "or" ? "or" : "and",
    conditions,
    groups
  };
}

function validatePolicyGroup(group, path = "根规则") {
  for (const [index, condition] of (group.conditions || []).entries()) {
    if (!String(condition.value || "").trim()) {
      throw new Error(`${path} 的第 ${index + 1} 个条件还没有填写值`);
    }
  }
  for (const [index, child] of (group.groups || []).entries()) {
    validatePolicyGroup(child, `${path} > 子组 ${index + 1}`);
  }
}

function normalizeStringList(value) {
  if (Array.isArray(value)) {
    return value.map((item) => String(item || "").trim()).filter(Boolean);
  }
  if (typeof value === "string") {
    try {
      const parsed = JSON.parse(value);
      if (Array.isArray(parsed)) {
        return parsed.map((item) => String(item || "").trim()).filter(Boolean);
      }
    } catch {
      return value
        .split(/\n+/)
        .map((item) => item.trim())
        .filter(Boolean);
    }
  }
  return [];
}

function syncAllowedUADraft(value) {
  const items = normalizeStringList(value);
  allowedUADraft.items.splice(0, allowedUADraft.items.length, ...items);
  allowedUADraft.pending = "";
  allowedUADraft.editIndex = -1;
  allowedUADraft.editValue = "";
}

function syncDiscordAccessPolicyDraft(value) {
  discordAccessPolicyDraft.value = parsePolicyValue(value);
}

function addAllowedUAItem() {
  const nextValue = allowedUADraft.pending.trim();
  if (!nextValue) {
    message.warning("请先输入一个 UA 前缀");
    return;
  }
  if (allowedUADraft.items.includes(nextValue)) {
    message.info("这个 UA 前缀已经存在");
    return;
  }
  allowedUADraft.items.push(nextValue);
  allowedUADraft.pending = "";
}

function startAllowedUAEdit(index) {
  allowedUADraft.editIndex = index;
  allowedUADraft.editValue = allowedUADraft.items[index] || "";
}

function cancelAllowedUAEdit() {
  allowedUADraft.editIndex = -1;
  allowedUADraft.editValue = "";
}

function saveAllowedUAEdit() {
  const nextValue = allowedUADraft.editValue.trim();
  if (allowedUADraft.editIndex < 0) return;
  if (!nextValue) {
    message.warning("UA 前缀不能为空");
    return;
  }
  if (allowedUADraft.items.some((item, index) => index !== allowedUADraft.editIndex && item === nextValue)) {
    message.info("这个 UA 前缀已经存在");
    return;
  }
  allowedUADraft.items.splice(allowedUADraft.editIndex, 1, nextValue);
  cancelAllowedUAEdit();
}

function removeAllowedUAItem(index) {
  allowedUADraft.items.splice(index, 1);
  if (allowedUADraft.editIndex === index) {
    cancelAllowedUAEdit();
  } else if (allowedUADraft.editIndex > index) {
    allowedUADraft.editIndex -= 1;
  }
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
  syncAllowedUADraft(payload.allowed_ua);
  settingsText.discord_oauth_scopes = formatMaybeArray(payload.discord_oauth_scopes);
  syncDiscordAccessPolicyDraft(payload.discord_access_policy);
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
    validatePolicyGroup(discordAccessPolicyDraft.value);
    payload.allowed_ua = [...allowedUADraft.items];
    payload.discord_oauth_scopes = settingsText.discord_oauth_scopes
      .split(/\n+/)
      .map((item) => item.trim())
      .filter(Boolean);
    payload.discord_access_policy = serializePolicyGroup(discordAccessPolicyDraft.value);

    await performPageTask(async () => {
      await request("/guard/api/settings", { method: "PUT", body: payload });
      await Promise.all([runViewLoad("settings"), runViewLoad("dashboard")]);
    }, "设置已保存");
  } catch (error) {
    message.error(error.message || "设置保存失败");
  }
}

function resetSettingsDraft() {
  syncAllowedUADraft(settingsModel.allowed_ua);
  settingsText.discord_oauth_scopes = formatMaybeArray(settingsModel.discord_oauth_scopes);
  syncDiscordAccessPolicyDraft(settingsModel.discord_access_policy);
  message.info("已恢复为当前已加载配置");
}

function switchBanMode(mode) {
  banMode.value = mode;
}

function trendWidth(value) {
  const max = Math.max(...statsLogs.value.map((item) => Number(item.total_requests || 0)), 1);
  return `${Math.max(8, Math.round((Number(value || 0) / max) * 100))}%`;
}

watch(themeMode, (mode) => {
  localStorage.setItem("guard_theme_mode", mode);
});

watch(
  resolvedTheme,
  (theme) => {
    applyResolvedTheme(theme);
  },
  { immediate: true }
);

watch(activeView, (view) => {
  ensureView(view).catch((error) => {
    message.error(error.message || "视图加载失败");
  });
});

onMounted(async () => {
  if (typeof window !== "undefined" && typeof window.matchMedia === "function") {
    colorSchemeMediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
    const syncSystemTheme = () => {
      systemTheme.value = colorSchemeMediaQuery.matches ? "dark" : "light";
    };
    syncSystemTheme();
    if (typeof colorSchemeMediaQuery.addEventListener === "function") {
      colorSchemeMediaQuery.addEventListener("change", syncSystemTheme);
      removeColorSchemeListener = () => colorSchemeMediaQuery.removeEventListener("change", syncSystemTheme);
    } else if (typeof colorSchemeMediaQuery.addListener === "function") {
      colorSchemeMediaQuery.addListener(syncSystemTheme);
      removeColorSchemeListener = () => colorSchemeMediaQuery.removeListener(syncSystemTheme);
    }
  }

  userSearchDraft.value = userQuery.search;
  if (auth.token) {
    try {
      await refreshAll();
    } catch {
      await logout(true);
    }
  }
});

onBeforeUnmount(() => {
  removeColorSchemeListener?.();
});
</script>
