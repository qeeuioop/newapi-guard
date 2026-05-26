<template>
  <n-config-provider :theme="naiveTheme" :theme-overrides="themeOverrides">
    <template v-if="!auth.token">
      <LoginView />
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

                <DashboardView v-if="activeView === 'dashboard'" />
                <UsersView v-else-if="activeView === 'users'" />
                <WhitelistView v-else-if="activeView === 'whitelist'" />
                <BansView v-else-if="activeView === 'bans'" />
                <SettingsView v-else-if="activeView === 'settings'" />
                <LogsView v-else-if="activeView === 'logs'" />
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
import { provide } from "vue";
import { useGuardStore } from "./composables/useGuardStore";

import LoginView from "./views/LoginView.vue";
import DashboardView from "./views/DashboardView.vue";
import UsersView from "./views/UsersView.vue";
import WhitelistView from "./views/WhitelistView.vue";
import BansView from "./views/BansView.vue";
import SettingsView from "./views/SettingsView.vue";
import LogsView from "./views/LogsView.vue";

const store = useGuardStore();
provide("guard", store);

const {
  // Theme (used by layout shell)
  themeOptions,
  themeMode,
  currentThemeLabel,
  naiveTheme,
  themeOverrides,

  // Navigation (used by sidebar)
  navItems,
  sectionMeta,
  activeView,
  sidebarCollapsed,
  openView,
  navCount,

  // Auth
  auth,
  logout,

  // Layout UI state
  pageLoading,
  currentViewError,
  dashboard,

  // Actions (used by header buttons)
  refreshCurrentView,
  refreshAll,

  // Modals (kept in App.vue since they overlay the whole layout)
  createUserModal,
  submitCreateUser,
  banModal,
  banDurationOptions,
  submitBan,
  helpModal,
  helpContent
} = store;
</script>
