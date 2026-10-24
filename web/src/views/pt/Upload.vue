<template>
  <div class="upload-page">
    <div class="top-nav">
      <n-button text @click="router.back()">
        <template #icon><n-icon><arrow-back-outline /></n-icon></template>
        返回种子列表
      </n-button>
    </div>

    <div class="form-card">
      <h2 class="form-title">发布新种子</h2>

      <n-form ref="formRef" :model="form" :rules="rules" label-placement="top">
        <n-form-item label="种子文件 (.torrent)" path="file">
          <n-upload
            :max="1"
            :default-upload="false"
            accept=".torrent"
            @change="handleFileChange"
          >
            <n-upload-dragger>
              <div style="margin-bottom: 12px">
                <n-icon size="48" :depth="3">
                  <cloud-upload-outline />
                </n-icon>
              </div>
              <n-text style="font-size: 16px">
                点击或者拖动种子文件到该区域上传
              </n-text>
              <n-p depth="3" style="margin: 8px 0 0 0">
                系统将自动强制设置 private=1 并配置本站 tracker
              </n-p>
            </n-upload-dragger>
          </n-upload>
        </n-form-item>

        <n-form-item label="种子主标题" path="name">
          <n-input
            v-model:value="form.name"
            placeholder="例如：Inception 2010 1080p BluRay x264-CHD"
          />
        </n-form-item>

        <n-form-item label="副标题 / 简要描述" path="small_descr">
          <n-input
            v-model:value="form.small_descr"
            placeholder="例如：盗梦空间 / 国英双语 / 简繁中字"
          />
        </n-form-item>

        <div class="grid-row">
          <n-form-item label="资源分类" path="category_id" style="flex: 1">
            <n-select
              v-model:value="form.category_id"
              :options="categoryOptions"
              placeholder="请选择分类"
            />
          </n-form-item>

          <n-form-item label="促销规则" path="sp_state" style="flex: 1">
            <n-select
              v-model:value="form.sp_state"
              :options="spStateOptions"
              placeholder="选择促销状态"
            />
          </n-form-item>
        </div>

        <n-form-item label="详细介绍 (支持排版 / Markdown)">
          <n-input
            v-model:value="form.descr"
            type="textarea"
            placeholder="请在此输入影片剧情简介、制作人员说明、技术参数等..."
            :rows="6"
          />
        </n-form-item>

        <n-form-item label="NFO 文件内容 (可选)">
          <n-input
            v-model:value="form.nfo"
            type="textarea"
            placeholder="粘贴 .nfo 文本内容"
            :rows="4"
          />
        </n-form-item>

        <n-form-item label="发布选项">
          <n-checkbox v-model:checked="form.is_anonymous">
            匿名发布（隐藏发布者用户名）
          </n-checkbox>
        </n-form-item>

        <div class="submit-row">
          <n-button
            type="primary"
            size="large"
            :loading="submitting"
            @click="handleSubmit"
          >
            立即发布种子
          </n-button>
        </div>
      </n-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue';
import { useRouter } from 'vue-router';
import {
  NButton,
  NIcon,
  NForm,
  NFormItem,
  NInput,
  NSelect,
  NCheckbox,
  NUpload,
  NUploadDragger,
  NText,
  NP,
  useMessage,
} from 'naive-ui';
import {
  ArrowBackOutline,
  CloudUploadOutline,
} from '@vicons/ionicons5';
import {
  getPTCategories,
  uploadTorrent,
  PTCategory,
} from '@/api/pt';

const router = useRouter();
const message = useMessage();

const categories = ref<PTCategory[]>([]);
const submitting = ref(false);
const selectedFile = ref<File | null>(null);

const form = reactive({
  name: '',
  small_descr: '',
  descr: '',
  category_id: 1,
  sp_state: 1,
  is_anonymous: false,
  nfo: '',
});

const rules = {
  name: { required: true, message: '请输入种子主标题', trigger: 'blur' },
  category_id: { required: true, type: 'number', message: '请选择分类', trigger: 'change' },
};

const categoryOptions = computed(() => {
  return categories.value.map((c) => ({
    label: c.name,
    value: c.id,
  }));
});

const spStateOptions = [
  { label: '普通 (1X 上传 / 1X 下载)', value: 1 },
  { label: '免费 (1X 上传 / 0X 下载)', value: 2 },
  { label: '双倍上传 (2X 上传 / 1X 下载)', value: 3 },
  { label: '双倍免费 (2X 上传 / 0X 下载)', value: 4 },
  { label: '50% 下载 (1X 上传 / 0.5X 下载)', value: 5 },
  { label: '双倍半价 (2X 上传 / 0.5X 下载)', value: 6 },
  { label: '30% 下载 (1X 上传 / 0.3X 下载)', value: 7 },
];

const handleFileChange = (data: { file: any; fileList: any[] }) => {
  if (data.fileList.length > 0) {
    const rawFile = data.fileList[0].file;
    selectedFile.value = rawFile;
    if (!form.name && rawFile.name) {
      form.name = rawFile.name.replace(/\.torrent$/i, '');
    }
  } else {
    selectedFile.value = null;
  }
};

const handleSubmit = async () => {
  if (!selectedFile.value) {
    message.warning('请先选择要上传的 .torrent 文件');
    return;
  }
  if (!form.name) {
    message.warning('请输入种子主标题');
    return;
  }

  submitting.value = true;
  try {
    const fd = new FormData();
    fd.append('file', selectedFile.value);
    fd.append('name', form.name);
    fd.append('small_descr', form.small_descr);
    fd.append('descr', form.descr);
    fd.append('category_id', String(form.category_id));
    fd.append('sp_state', String(form.sp_state));
    fd.append('is_anonymous', form.is_anonymous ? '1' : '0');
    fd.append('nfo', form.nfo);

    const res = await uploadTorrent(fd);
    message.success('种子发布成功！');
    router.push({ path: '/pt/detail', query: { id: res.id } });
  } catch (e: any) {
    message.error(e?.msg || '发布失败');
  } finally {
    submitting.value = false;
  }
};

onMounted(async () => {
  try {
    categories.value = await getPTCategories();
    if (categories.value.length > 0) {
      form.category_id = categories.value[0].id;
    }
  } catch (e) {}
});
</script>

<style lang="less" scoped>
.upload-page {
  padding: 16px;
  max-width: 800px;
  margin: 0 auto;

  .top-nav {
    margin-bottom: 14px;
  }

  .form-card {
    background: var(--n-color);
    border-radius: 8px;
    padding: 24px;
    border: 1px solid rgba(0, 0, 0, 0.06);

    .form-title {
      margin: 0 0 20px 0;
      font-size: 20px;
      font-weight: 600;
    }

    .grid-row {
      display: flex;
      gap: 16px;
    }

    .submit-row {
      display: flex;
      justify-content: flex-end;
      margin-top: 16px;
    }
  }
}
</style>
