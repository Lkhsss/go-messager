// Gravatar 地址拼接。网站不保存头像图片，浏览器直接向 gravatar.com 请求。
// d=identicon：该邮箱没有注册 Gravatar 时返回一个生成的图案，而不是 404。
const GRAVATAR_BASE = "https://www.gravatar.com/avatar";

export function gravatarUrl(hash: string | undefined | null, size = 80): string | null {
  if (!hash) {
    return null;
  }
  return `${GRAVATAR_BASE}/${hash}?d=identicon&s=${size}`;
}
