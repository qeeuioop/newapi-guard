<template>
  <section class="section-stack">
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
</template>

<script setup>
import { inject } from 'vue'

const store = inject('guard')
const {
  dashboard,
  settingsModel,
  activeBans,
  statsLogs,
  currentViewHealth,
  currentViewSyncedAt,
  formatNumber,
  trendWidth,
  openView,
  quickUnban
} = store
</script>
