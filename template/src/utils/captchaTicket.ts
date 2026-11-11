// 人机验证 ticket 只用于本次注册，放 sessionStorage，关闭标签页即失效。
const STORAGE_KEY = "go-messager:captcha-ticket";

interface StoredTicket {
  ticket: string;
  // 毫秒时间戳。
  expiresAt: number;
}

export function saveCaptchaTicket(ticket: string, expiresInSeconds: number): void {
  try {
    const payload: StoredTicket = {
      ticket,
      expiresAt: Date.now() + Math.max(0, expiresInSeconds) * 1000,
    };
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(payload));
  } catch {
    // sessionStorage 不可用时忽略，注册时会重新引导验证。
  }
}

// 读取有效 ticket；过期或损坏时顺手清掉。
export function readCaptchaTicket(): string | null {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    if (!raw) return null;

    const parsed = JSON.parse(raw) as Partial<StoredTicket> | null;
    if (
      typeof parsed?.ticket !== "string" ||
      !parsed.ticket ||
      typeof parsed.expiresAt !== "number" ||
      parsed.expiresAt <= Date.now()
    ) {
      sessionStorage.removeItem(STORAGE_KEY);
      return null;
    }
    return parsed.ticket;
  } catch {
    return null;
  }
}

export function clearCaptchaTicket(): void {
  try {
    sessionStorage.removeItem(STORAGE_KEY);
  } catch {
    // 忽略
  }
}
