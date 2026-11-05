import { ref } from "vue";
import { defineStore } from "pinia";
import { getStatus } from "@/api/system";

export const useSystemStore = defineStore("system", () => {
  // null 表示还没查过；false 表示库里没有用户，需要进入初始化流程
  const initialized = ref<boolean | null>(null);
  let inflight: Promise<boolean> | null = null;

  // 查询系统是否已初始化。同一页面生命周期内只请求一次，并发调用共用同一个 Promise。
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
        // 后端未启动/网络异常时不要把用户导到初始化页：当作已初始化，交给登录页提示错误，
        // 并且不缓存结果，下次导航会重试。
        return true;
      })
      .finally(() => {
        inflight = null;
      });

    return inflight;
  }

  // 初始化成功（创建了第一个管理员）后调用
  function markInitialized() {
    initialized.value = true;
  }

  return { initialized, ensureInitialized, markInitialized };
});
