<template>
  <section class="section-stack">
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
                  <div class="cell-title">{{ item.username || item.display_name || item.discord_name || "未命名用户" }}</div>
                  <div class="cell-sub">昵称：{{ item.display_name || item.discord_name || item.username || "未设置" }}</div>
                  <div class="cell-sub">Discord：{{ item.discord_name || item.discord_id || "未记录" }}</div>
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
</template>

<script setup>
import { inject } from 'vue'

const store = inject('guard')
const { whitelist, parseDate, toggleWhitelist } = store
</script>
