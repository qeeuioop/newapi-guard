<template>
  <section class="section-stack">
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
                      <div class="cell-title">{{ item.username || item.display_name || item.discord_name || "未命名用户" }}</div>
                      <div class="cell-sub">昵称：{{ item.display_name || item.discord_name || item.username || "未设置" }}</div>
                      <div class="cell-sub">Discord：{{ item.discord_name || item.discord_id || "未记录" }}</div>
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
            <input v-model="checkinFilter" class="app-input filter-input" type="text" placeholder="按用户名、昵称、Discord 或用户 ID 过滤" @keyup.enter="loadCheckinLogsAction" />
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
                  <td>
                    <div class="cell-stack">
                      <div class="cell-title">{{ item.username || item.display_name || item.discord_name || "未命名用户" }}</div>
                      <div class="cell-sub">昵称：{{ item.display_name || item.discord_name || item.username || "未设置" }}</div>
                      <div class="cell-sub">Discord：{{ item.discord_name || item.discord_id || "未记录" }}</div>
                    </div>
                  </td>
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
</template>

<script setup>
import { inject } from 'vue'

const store = inject('guard')
const {
  logView,
  checkinFilter,
  banLogs,
  checkinLogs,
  statsLogs,
  formatNumber,
  parseDate,
  refreshLogsOnly,
  loadCheckinLogsAction
} = store
</script>
