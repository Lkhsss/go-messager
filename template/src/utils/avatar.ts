// 头像地址，版本号用于破缓存。
export function avatarUrl(name: string, version?: string | null): string | null {
  if (!name || !version) {
    return null;
  }
  return `/api/avatar?name=${encodeURIComponent(name)}&v=${encodeURIComponent(version)}`;
}

const MAX_FILE_BYTES = 5 * 1024 * 1024;
const AVATAR_SIZE = 256;

function loadImage(file: File): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file);
    const img = new Image();
    img.onload = () => {
      URL.revokeObjectURL(url);
      resolve(img);
    };
    img.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error("图片解析失败，请换一张试试"));
    };
    img.src = url;
  });
}

// 居中裁剪并缩放到 256×256，输出 JPEG。
export async function fileToAvatarDataUrl(file: File, size = AVATAR_SIZE): Promise<string> {
  if (!/^image\/(jpeg|png)$/.test(file.type)) {
    throw new Error("只支持 JPEG/PNG 图片");
  }
  if (file.size > MAX_FILE_BYTES) {
    throw new Error("图片不能超过 5MB");
  }

  const img = await loadImage(file);

  const canvas = document.createElement("canvas");
  canvas.width = size;
  canvas.height = size;

  const ctx = canvas.getContext("2d");
  if (!ctx) {
    throw new Error("当前浏览器不支持图片处理");
  }

  // 白底，避免透明区域变黑。
  ctx.fillStyle = "#ffffff";
  ctx.fillRect(0, 0, size, size);

  const side = Math.min(img.naturalWidth, img.naturalHeight);
  const sx = (img.naturalWidth - side) / 2;
  const sy = (img.naturalHeight - side) / 2;
  ctx.drawImage(img, sx, sy, side, side, 0, 0, size, size);

  return canvas.toDataURL("image/jpeg", 0.85);
}
