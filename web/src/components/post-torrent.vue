<template>
  <div class="post-torrent-card" @click.stop>
    <div class="torrent-info-row">
      <div class="torrent-icon">
        <n-icon size="28" color="#18a058">
          <cloud-download-outline />
        </n-icon>
      </div>

      <div class="torrent-meta">
        <div class="torrent-title-row">
          <span class="torrent-name" :title="torrent.name">
            {{ torrent.name }}
          </span>
          <n-tag
            class="sp-badge"
            :type="getSpType(torrent.sp_state)"
            size="small"
            round
          >
            {{ getSpLabel(torrent.sp_state) }}
          </n-tag>
        </div>

        <div class="torrent-stats-row">
          <span class="stat-item size">
            <n-icon size="13"><server-outline /></n-icon>
            {{ formatSize(torrent.size) }}
          </span>
          <span class="dot">·</span>
          <span class="stat-item sp-stat">
            <n-icon size="13"><pricetag-outline /></n-icon>
            促销: <strong :style="{ color: torrent.sp_state > 1 ? '#d03050' : 'inherit' }">{{ getSpLabel(torrent.sp_state) }}</strong>
          </span>
          <span class="dot">·</span>
          <span class="stat-item seeders">
            做种 <strong class="num">{{ torrent.seeders }}</strong>
          </span>
          <span class="dot">·</span>
          <span class="stat-item leechers">
            下载 <strong class="num">{{ torrent.leechers }}</strong>
          </span>
          <span class="dot">·</span>
          <span class="stat-item snatches">
            完成 <strong class="num">{{ torrent.times_completed }}</strong>
          </span>
        </div>
      </div>
    </div>

    <div class="torrent-actions-row">
      <n-button
        type="success"
        secondary
        size="small"
        @click="toggleWebPlayer"
      >
        <template #icon><n-icon><play-circle-outline /></n-icon></template>
        {{ showPlayer ? '收起播放' : '网页播放' }}
      </n-button>
      <n-button
        type="primary"
        secondary
        size="small"
        @click="showFilesModal = true"
      >
        文件清单
      </n-button>
      <n-button
        type="primary"
        size="small"
        @click="handleDownload"
      >
        <template #icon><n-icon><arrow-down-circle-outline /></n-icon></template>
        下载种子
      </n-button>
    </div>

    <!-- Inline WebTorrent Streaming Player Area -->
    <div v-if="showPlayer" class="webtorrent-player-box">
      <div class="player-toolbar">
        <div class="stream-stats">
          <n-tag size="tiny" type="success" round>WebTorrent P2P</n-tag>
          <span class="stat">节点: <strong>{{ p2pPeers }}</strong></span>
          <span class="stat">下载: <strong>{{ p2pDownSpeed }}</strong></span>
          <span class="stat">上传: <strong>{{ p2pUpSpeed }}</strong></span>
          <span class="stat">进度: <strong>{{ (p2pProgress * 100).toFixed(1) }}%</strong></span>
        </div>
        <n-button size="tiny" quaternary type="error" @click="stopPlayer">
          关闭播放
        </n-button>
      </div>

      <div class="player-media-wrap">
        <n-spin :show="playerLoading" description="正在通过 WebRTC 寻找节点并缓冲媒体流...">
          <div ref="mediaContainerRef" class="media-container"></div>
        </n-spin>
      </div>
      <n-progress
        type="line"
        :percentage="Math.round(p2pProgress * 100)"
        :show-indicator="false"
        status="success"
        style="margin-top: 6px"
      />
    </div>

    <!-- Modal for File List & Swarm Peers -->
    <n-modal
      v-model:show="showFilesModal"
      preset="card"
      title="种子详情与文件清单"
      style="width: 650px; max-width: 90vw"
      @after-enter="loadDetails"
    >
      <n-spin :show="detailLoading">
        <div class="modal-info-summary">
          <div><strong>散列值 (Hash):</strong> <code>{{ torrent.info_hash }}</code></div>
          <div style="margin-top: 6px">
            <strong>文件总数:</strong> {{ files.length || torrent.num_files }} 个文件
            <span style="margin-left: 12px"><strong>总大小:</strong> {{ formatSize(torrent.size) }}</span>
          </div>
        </div>

        <n-table :single-line="false" size="small" style="margin-top: 14px">
          <thead>
            <tr>
              <th>文件名 / 路径</th>
              <th style="width: 120px; text-align: right">大小</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="f in files" :key="f.id">
              <td>{{ f.file_path }}</td>
              <td style="text-align: right">{{ formatSize(f.size) }}</td>
            </tr>
          </tbody>
        </n-table>
      </n-spin>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, onUnmounted } from 'vue';
import {
  NButton,
  NIcon,
  NTag,
  NModal,
  NSpin,
  NTable,
  NProgress,
  useMessage,
} from 'naive-ui';
import {
  CloudDownloadOutline,
  ServerOutline,
  ArrowDownCircleOutline,
  PricetagOutline,
  PlayCircleOutline,
} from '@vicons/ionicons5';
import {
  getTorrentDetail,
  getTorrentDownloadURL,
  getSpLabel,
  getSpType,
  TorrentItem,
  TorrentFile,
} from '@/api/pt';

const props = defineProps<{
  torrent: TorrentItem;
}>();

const message = useMessage();
const showFilesModal = ref(false);
const detailLoading = ref(false);
const files = ref<TorrentFile[]>([]);

const formatSize = (bytes: number) => {
  if (!bytes || bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return (bytes / Math.pow(k, i)).toFixed(2) + ' ' + sizes[i];
};

const handleDownload = () => {
  const url = getTorrentDownloadURL(props.torrent.id);
  window.open(url, '_blank');
  message.success('已开始下载专属种子');
};

let wtClient: any = null;
const showPlayer = ref(false);
const playerLoading = ref(false);
const mediaContainerRef = ref<HTMLElement | null>(null);
const p2pPeers = ref(0);
const p2pDownSpeed = ref('0 B/s');
const p2pUpSpeed = ref('0 B/s');
const p2pProgress = ref(0);
let statsInterval: any = null;

const toggleWebPlayer = async () => {
  if (showPlayer.value) {
    stopPlayer();
    return;
  }
  showPlayer.value = true;
  playerLoading.value = true;

  try {
    const { default: WebTorrent } = await import('webtorrent/dist/webtorrent.min.js');
    if (!wtClient) {
      wtClient = new WebTorrent();
    }

    const downloadUrl = getTorrentDownloadURL(props.torrent.id);

    wtClient.add(downloadUrl, (torrent: any) => {
      playerLoading.value = false;

      // Find first playable media file
      const file = torrent.files.find((f: any) => {
        return /\.(mp4|webm|mkv|mov|mp3|wav|ogg)$/i.test(f.name);
      }) || torrent.files[0];

      if (file && mediaContainerRef.value) {
        mediaContainerRef.value.innerHTML = '';
        file.renderTo(mediaContainerRef.value, {
          autoplay: true,
          controls: true,
        }, (err: any) => {
          if (err) {
            console.error('renderTo error:', err);
          }
        });
      }

      statsInterval = setInterval(() => {
        p2pPeers.value = torrent.numPeers || 0;
        p2pDownSpeed.value = formatSize(torrent.downloadSpeed) + '/s';
        p2pUpSpeed.value = formatSize(torrent.uploadSpeed) + '/s';
        p2pProgress.value = torrent.progress || 0;
      }, 1000);
    });
  } catch (err: any) {
    playerLoading.value = false;
    message.error('启动 WebTorrent 播放失败: ' + (err.message || err));
  }
};

const stopPlayer = () => {
  showPlayer.value = false;
  playerLoading.value = false;
  if (statsInterval) {
    clearInterval(statsInterval);
    statsInterval = null;
  }
  if (wtClient) {
    try {
      wtClient.destroy();
    } catch (e) {}
    wtClient = null;
  }
  if (mediaContainerRef.value) {
    mediaContainerRef.value.innerHTML = '';
  }
  p2pPeers.value = 0;
  p2pProgress.value = 0;
  p2pDownSpeed.value = '0 B/s';
  p2pUpSpeed.value = '0 B/s';
};

onUnmounted(() => {
  stopPlayer();
});

const loadDetails = async () => {
  if (files.value.length > 0) return;
  detailLoading.value = true;
  try {
    const res = await getTorrentDetail(props.torrent.id);
    files.value = res.files || [];
  } catch (e) {
    console.error(e);
  } finally {
    detailLoading.value = false;
  }
};
</script>

<style lang="less" scoped>
.post-torrent-card {
  margin-top: 10px;
  margin-bottom: 8px;
  background: rgba(24, 160, 88, 0.04);
  border: 1px solid rgba(24, 160, 88, 0.2);
  border-radius: 8px;
  padding: 12px 14px;
  transition: all 0.2s;

  &:hover {
    border-color: rgba(24, 160, 88, 0.4);
    background: rgba(24, 160, 88, 0.07);
  }

  .torrent-info-row {
    display: flex;
    align-items: center;
    gap: 12px;
    width: 100%;

    .torrent-icon {
      flex-shrink: 0;
      display: flex;
      align-items: center;
      justify-content: center;
      width: 44px;
      height: 44px;
      background: #e8f7ee;
      border-radius: 8px;
    }

    .torrent-meta {
      flex: 1;
      min-width: 0;

      .torrent-title-row {
        display: flex;
        align-items: center;
        gap: 8px;

        .torrent-name {
          font-size: 14px;
          font-weight: 600;
          color: var(--n-text-color);
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
        }

        .sp-badge {
          font-weight: 700;
          text-transform: uppercase;
          flex-shrink: 0;
        }
      }

      .torrent-stats-row {
        display: flex;
        align-items: center;
        flex-wrap: wrap;
        gap: 6px;
        font-size: 12px;
        color: #777;
        margin-top: 5px;

        .dot {
          color: #ccc;
        }

        .stat-item {
          display: inline-flex;
          align-items: center;
          gap: 3px;

          &.seeders .num {
            color: #18a058;
          }
          &.leechers .num {
            color: #d03050;
          }
          &.snatches .num {
            color: #2080f0;
          }
        }
      }
    }
  }

  .torrent-actions-row {
    display: flex;
    justify-content: flex-end;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
    margin-top: 10px;
    padding-top: 8px;
    border-top: 1px dashed rgba(24, 160, 88, 0.15);
  }

  .webtorrent-player-box {
    margin-top: 12px;
    padding: 10px;
    background: rgba(0, 0, 0, 0.03);
    border-radius: 8px;
    border: 1px solid rgba(24, 160, 88, 0.2);

    .player-toolbar {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 8px;

      .stream-stats {
        display: flex;
        align-items: center;
        gap: 10px;
        font-size: 11px;
        opacity: 0.85;

        .stat strong {
          color: #18a058;
        }
      }
    }

    .player-media-wrap {
      min-height: 180px;
      display: flex;
      align-items: center;
      justify-content: center;
      background: #000;
      border-radius: 6px;
      overflow: hidden;

      .media-container {
        width: 100%;
        display: flex;
        justify-content: center;

        video, audio {
          width: 100%;
          max-height: 480px;
        }
      }
    }
  }

  .modal-info-summary {
    background: rgba(0, 0, 0, 0.02);
    padding: 12px;
    border-radius: 6px;
    font-size: 13px;

    code {
      font-family: monospace;
      color: #18a058;
    }
  }
}
</style>
