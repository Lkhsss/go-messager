import { computed, ref } from "vue";
import { defineStore } from "pinia";
import { getStatus, type AccessMode, type MaintenanceInfo, type SystemStatus } from "@/api/system";

// 默认关闭，接口异常时按正常运行处理。
const DEFAULT_MAINTENANCE: MaintenanceInfo = { enabled: false, title: "", body: "" };

export const useSystemStore = defineStore("system", () => {
  // null 表示未查询。
  const initialized = ref<boolean | null>(null);
  // 默认 private，接口异常时保持最保守的行为。
  const mode = ref<AccessMode>("private");
  const maintenance = ref<MaintenanceInfo>({ ...DEFAULT_MAINTENANCE });
  const isPublic = computed(() => mode.value === "public");
  const isMaintenance = computed(() => maintenance.value.enabled);
  let inflight: Promise<boolean> | null = null;

  // 写入一次接口返回的站点状态。
  function applyStatus(status: SystemStatus) {
    initialized.value = status.initialized;
    mode.value = status.mode === "public" ? "public" : "private";
    maintenance.value = { ...DEFAULT_MAINTENANCE, ...(status.maintenance ?? {}) };
  }

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
        applyStatus(status);
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

  // 强制重新拉取状态：维护页轮询、遇到维护拦截后使用。
  async function refreshStatus(): Promise<void> {
    try {
      applyStatus(await getStatus());
    } catch {
      // 网络异常时保持现有状态。
    }
  }

  // 管理员保存维护配置后同步本地状态。
  function applyMaintenance(next: MaintenanceInfo) {
    maintenance.value = { ...DEFAULT_MAINTENANCE, ...next };
  }

  // 初始化成功后调用。
  function markInitialized() {
    initialized.value = true;
  }

  return {
    initialized,
    mode,
    maintenance,
    isPublic,
    isMaintenance,
    ensureInitialized,
    refreshStatus,
    applyMaintenance,
    markInitialized,
  };
});
