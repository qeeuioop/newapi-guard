<template>
  <section class="section-stack">
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
</template>

<script setup>
import { inject } from 'vue'

const store = inject('guard')
const {
  banMode,
  banFilter,
  activeBans,
  banHistory,
  activeBansWithContext,
  displayedBans,
  filteredBans,
  formatNumber,
  parseDate,
  switchBanMode,
  openBanModal,
  quickUnban
} = store
</script>
