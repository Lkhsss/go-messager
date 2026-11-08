import { ref } from "vue";
import { defineStore } from "pinia";
import { getStatus } from "@/api/system";

export const useSystemStore = defineStore("system", () => {
  // null 表示未查询。
  const initialized = ref<boolean | null>(null);
  let inflight: Promise<boolean> | null = null;

  // 只请求一次，并发复用。
  async function ensureInitialized(): Promise<boolean> {
    if (initialized.value !== null) {
      return initialized.value;
    }
    if (inflight) {
      return inflight;
    }

    inflight = getStatus()
      .then((status) => {
        initialized.value = status.initialized;
        return status.initialized;
      })
      .catch(() => {
        // 网络异常时当作已初始化，不缓存结果。
        return true;
      })
      .finally(() => {
        inflight = null;
      });

    return inflight;
  }

  // 初始化成功后调用。
  function markInitialized() {
    initialized.value = true;
  }

  return { initialized, ensureInitialized, markInitialized };
});
