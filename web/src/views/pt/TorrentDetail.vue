<template>
  <div class="torrent-detail-page">
    <div class="top-nav">
      <n-button text @click="router.back()">
        <template #icon><n-icon><arrow-back-outline /></n-icon></template>
        返回种子列表
      </n-button>
    </div>

    <n-spin :show="loading">
      <div v-if="torrent" class="detail-container">
        <!-- Main Header Card -->
        <div class="torrent-header-card">
          <div class="title-section">
            <h1 class="main-title">
              {{ torrent.name }}
              <span v-if="torrent.sp_state > 1" class="sp-tag" :class="'sp-' + torrent.sp_state">
                {{ getSpLabel(torrent.sp_state) }}
              </span>
            </h1>
            <div v-if="torrent.small_descr" class="sub-title">{{ torrent.small_descr }}</div>
          </div>

          <div class="quick-stats-grid">
            <div class="stat-box">
              <span class="label">大小</span>
              <span class="value">{{ formatSize(torrent.size) }}</span>
            </div>
            <div class="stat-box">
              <span class="label">分类</span>
              <span class="value">{{ torrent.category?.name || '未知' }}</span>
            </div>
            <div class="stat-box">
              <span class="label">做种人数</span>
              <span class="value seeders">{{ torrent.seeders }}</span>
            </div>
            <div class="stat-box">
              <span class="label">下载人数</span>
              <span class="value leechers">{{ torrent.leechers }}</span>
            </div>
            <div class="stat-box">
              <span class="label">完成次数</span>
              <span class="value">{{ torrent.times_completed }}</span>
            </div>
            <div class="stat-box">
              <span class="label">发布者</span>
              <span class="value">{{ torrent.is_anonymous ? '匿名用户' : (torrent.nickname || torrent.username || '用户') }}</span>
            </div>
          </div>

          <div class="action-bar">
            <n-button type="primary" size="large" @click="downloadTorrent">
              <template #icon><n-icon><arrow-down-circle-outline /></n-icon></template>
              下载种子 (.torrent)
            </n-button>
            <div class="hash-display">
              <span class="hash-label">Info Hash:</span>
              <code class="hash-code">{{ torrent.info_hash }}</code>
            </div>
          </div>
        </div>

        <!-- Detail Tabs -->
        <div class="content-tabs-card">
          <n-tabs type="line" animated>
            <n-tab-pane name="descr" tab="种子介绍">
              <div class="descr-content" v-html="formattedDescr"></div>
            </n-tab-pane>

            <n-tab-pane name="files" :tab="`文件列表 (${files.length})`">
              <n-table :single-line="false" size="small">
                <thead>
                  <tr>
                    <th>文件名 / 路径</th>
                    <th style="width: 140px; text-align: right;">文件大小</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="f in files" :key="f.id">
                    <td>{{ f.file_path }}</td>
                    <td style="text-align: right;">{{ formatSize(f.size) }}</td>
                  </tr>
                </tbody>
              </n-table>
            </n-tab-pane>

            <n-tab-pane name="peers" :tab="`当前节点 (${peers.length})`">
              <n-empty v-if="peers.length === 0" description="当前没有活跃对等节点连接到 Tracker" />
              <n-table v-else :single-line="false" size="small">
                <thead>
                  <tr>
                    <th>客户端 Agent</th>
                    <th>类型</th>
                    <th>进度</th>
                    <th style="text-align: right;">上传量</th>
                    <th style="text-align: right;">下载量</th>
                    <th style="text-align: right;">最后汇报</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="p in peers" :key="p.id">
                    <td>{{ p.agent || p.peer_id }}</td>
                    <td>
                      <n-tag size="tiny" :type="p.is_seeder ? 'success' : 'warning'">
                        {{ p.is_seeder ? '做种中' : '下载中' }}
                      </n-tag>
                    </td>
                    <td>
                      {{ p.is_seeder ? '100%' : (torrent.size > 0 ? ((1 - p.to_go / torrent.size) * 100).toFixed(1) + '%' : '0%') }}
                    </td>
                    <td style="text-align: right;">{{ formatSize(p.uploaded) }}</td>
                    <td style="text-align: right;">{{ formatSize(p.downloaded) }}</td>
                    <td style="text-align: right;">{{ formatRelative(p.last_action) }}</td>
                  </tr>
                </tbody>
              </n-table>
            </n-tab-pane>

            <n-tab-pane v-if="torrent.nfo" name="nfo" tab="NFO 信息">
              <pre class="nfo-view">{{ torrent.nfo }}</pre>
            </n-tab-pane>
          </n-tabs>
        </div>
      </div>
    </n-spin>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import {
  NButton,
  NIcon,
  NSpin,
  NTabs,
  NTabPane,
  NTable,
  NTag,
  NEmpty,
} from 'naive-ui';
import {
  ArrowBackOutline,
  ArrowDownCircleOutline,
} from '@vicons/ionicons5';
import moment from 'moment';
import {
  getTorrentDetail,
  getTorrentDownloadURL,
  TorrentItem,
  TorrentFile,
  TorrentPeer,
} from '@/api/pt';

const route = useRoute();
const router = useRouter();

const loading = ref(false);
const torrent = ref<TorrentItem | null>(null);
const files = ref<TorrentFile[]>([]);
const peers = ref<TorrentPeer[]>([]);

const formatSize = (bytes: number) => {
  if (!bytes || bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return (bytes / Math.pow(k, i)).toFixed(2) + ' ' + sizes[i];
};

const formatRelative = (ts: number) => {
  if (!ts) return '';
  return moment(ts * 1000).fromNow();
};

const getSpLabel = (sp: number) => {
  switch (sp) {
    case 2: return 'Free';
    case 3: return '2X';
    case 4: return '2X Free';
    case 5: return '50%';
    case 6: return '2X 50%';
    case 7: return '30%';
    default: return '';
  }
};

const formattedDescr = computed(() => {
  if (!torrent.value?.descr) return '暂无介绍';
  return torrent.value.descr.replace(/\n/g, '<br/>');
});

const downloadTorrent = () => {
  if (torrent.value) {
    const url = getTorrentDownloadURL(torrent.value.id);
    window.open(url, '_blank');
  }
};

onMounted(async () => {
  const id = Number(route.query.id);
  if (!id) {
    router.replace('/pt');
    return;
  }

  loading.value = true;
  try {
    const res = await getTorrentDetail(id);
    torrent.value = res.torrent;
    files.value = res.files || [];
    peers.value = res.peers || [];
  } catch (e) {
    console.error(e);
  } finally {
    loading.value = false;
  }
});
</script>

<style lang="less" scoped>
.torrent-detail-page {
  padding: 16px;
  max-width: 1200px;
  margin: 0 auto;

  .top-nav {
    margin-bottom: 14px;
  }

  .torrent-header-card {
    background: var(--n-color);
    border-radius: 8px;
    padding: 20px;
    margin-bottom: 16px;
    border: 1px solid rgba(0, 0, 0, 0.06);

    .title-section {
      margin-bottom: 16px;

      .main-title {
        margin: 0;
        font-size: 20px;
        font-weight: 700;
        display: flex;
        align-items: center;
        gap: 10px;
        word-break: break-all;

        .sp-tag {
          font-size: 12px;
          padding: 2px 8px;
          border-radius: 4px;
          font-weight: 600;

          &.sp-2 {
            background: #e6f7ff;
            color: #1890ff;
          }
          &.sp-3 {
            background: #fff7e6;
            color: #fa8c16;
          }
          &.sp-4 {
            background: #f6ffed;
            color: #52c41a;
          }
        }
      }

      .sub-title {
        color: #777;
        margin-top: 6px;
        font-size: 14px;
      }
    }

    .quick-stats-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));
      gap: 12px;
      padding: 14px;
      background: rgba(0, 0, 0, 0.02);
      border-radius: 6px;
      margin-bottom: 20px;

      .stat-box {
        display: flex;
        flex-direction: column;

        .label {
          font-size: 12px;
          color: #888;
        }

        .value {
          font-size: 15px;
          font-weight: 600;
          margin-top: 2px;

          &.seeders {
            color: #18a058;
          }
          &.leechers {
            color: #d03050;
          }
        }
      }
    }

    .action-bar {
      display: flex;
      justify-content: space-between;
      align-items: center;
      flex-wrap: wrap;
      gap: 12px;

      .hash-display {
        font-size: 13px;
        color: #777;

        .hash-code {
          margin-left: 6px;
          font-family: monospace;
          background: rgba(0, 0, 0, 0.04);
          padding: 2px 6px;
          border-radius: 4px;
        }
      }
    }
  }

  .content-tabs-card {
    background: var(--n-color);
    border-radius: 8px;
    padding: 20px;
    border: 1px solid rgba(0, 0, 0, 0.06);

    .descr-content {
      line-height: 1.8;
      font-size: 14px;
      padding: 10px 0;
    }

    .nfo-view {
      font-family: monospace;
      font-size: 12px;
      background: #111;
      color: #0f0;
      padding: 16px;
      border-radius: 6px;
      overflow-x: auto;
    }
  }
}
</style>
