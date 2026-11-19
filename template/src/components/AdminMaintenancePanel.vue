<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { fetchMaintenance, saveMaintenance } from "@/api/adminMaintenance";
import { isApiError } from "@/api/http";
import { useSystemStore } from "@/stores/system";

const TITLE_MAX = 60;
const BODY_MAX = 2000;

const system = useSystemStore();

const enabled = ref(false);
const title = ref("");
const body = ref("");
const loading = ref(false);
const saving = ref(false);
const errorMsg = ref("");
const successMsg = ref("");

const statusText = computed(() =>
  system.isMaintenance ? "维护模式已开启：普通用户当前无法访问站点" : "站点正常运行",
);

async function load() {
  loading.value = true;
  errorMsg.value = "";
  try {
    const state = await fetchMaintenance();
    enabled.value = state.enabled;
    title.value = state.title;
    body.value = state.body;
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : "获取维护模式配置失败，请稍后再试";
  } finally {
    loading.value = false;
  }
}

async function onSave() {
  if (saving.value) return;

  saving.value = true;
  errorMsg.value = "";
  successMsg.value = "";
  try {
    const state = await saveMaintenance({
      enabled: enabled.value,
      title: title.value.trim(),
      body: body.value.trim(),
    });
    enabled.value = state.enabled;
    title.value = state.title;
    body.value = state.body;
    system.applyMaintenance(state);
    successMsg.value = state.enabled
      ? "维护模式已开启，普通用户现在只能看到维护页"
      : "维护模式已关闭，站点已恢复";
  } catch (err) {
    errorMsg.value = isApiError(err) ? err.message : "保存失败，请稍后再试";
  } finally {
    saving.value = false;
  }
}

onMounted(() => {
  void load();
});
</script>

<template>
  <div class="card bg-base-100 shadow-sm">
    <div class="card-body gap-4">
      <div>
        <h2 class="card-title text-lg">维护模式</h2>
        <p class="mt-1 text-sm text-base-content/60">
          开启后普通用户无法登录、注册和访问任何接口，只有管理员可以继续使用并在本页关闭。
        </p>
      </div>

      <div
        role="status"
        class="alert py-2 text-sm"
        :class="system.isMaintenance ? 'alert-warning alert-soft' : 'alert-success alert-soft'"
      >
        <span>{{ statusText }}</span>
      </div>

      <label class="flex cursor-pointer items-center gap-3 text-sm">
        <input
          v-model="enabled"
          type="checkbox"
          class="toggle toggle-warning"
          :disabled="loading || saving"
        />
        <span>开启维护模式</span>
      </label>

      <label class="floating-label">
        <span>标题（留空使用「站点维护中」）</span>
        <input
          v-model="title"
          type="text"
          :maxlength="TITLE_MAX"
          placeholder="站点维护中"
          class="input input-bordered w-full"
          :disabled="loading || saving"
        />
      </label>

      <label class="floating-label">
        <span>正文（可选，留空时只显示大号标题）</span>
        <textarea
          v-model="body"
          :maxlength="BODY_MAX"
          rows="5"
          placeholder="例如：预计 22:00 恢复，请稍后再来。"
          class="textarea textarea-bordered w-full"
          :disabled="loading || saving"
        ></textarea>
      </label>

      <div class="flex flex-wrap items-center justify-between gap-2 text-xs text-base-content/50">
        <span>正文 {{ body.length }} / {{ BODY_MAX }} 字</span>
        <span>维护页每 15 秒自动检测，关闭后立即恢复</span>
      </div>

      <div v-if="errorMsg" role="alert" class="alert alert-error alert-soft py-2 text-sm">
        <span>{{ errorMsg }}</span>
      </div>
      <div v-if="successMsg" role="status" class="alert alert-success alert-soft py-2 text-sm">
        <span>{{ successMsg }}</span>
      </div>

      <div class="flex justify-end">
        <button type="button" class="btn btn-primary" :disabled="loading || saving" @click="onSave">
          <span v-if="saving" class="loading loading-spinner loading-sm"></span>
          保存
        </button>
      </div>
    </div>
  </div>
</template>
