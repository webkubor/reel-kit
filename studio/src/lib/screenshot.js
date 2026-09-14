/**
 * 通用截图工具 —— 基于 html2canvas 截 DOM 元素为 PNG
 *
 * 用法：
 *   const blob = await screenshot(element);
 *   downloadBlob(blob, 'card.png');
 *   或
 *   await api.uploadCapture(blob, meta);
 */
import html2canvas from 'html2canvas';

const DEFAULT_OPTIONS = {
  backgroundColor: '#0b0b0d', // 暗夜黑金底色
  scale: 2, // 高清输出(2x)
  logging: false,
  useCORS: true,
  allowTaint: true,
};

/** 截 DOM 元素为 PNG Blob */
export async function screenshot(el, options = {}) {
  if (!el) throw new Error('screenshot: element is required');
  const canvas = await html2canvas(el, { ...DEFAULT_OPTIONS, ...options });
  return new Promise((resolve) => {
    canvas.toBlob((blob) => resolve(blob), 'image/png');
  });
}

/** 截 + 触发浏览器下载 */
export async function screenshotDownload(el, filename = 'capture.png', options) {
  const blob = await screenshot(el, options);
  downloadBlob(blob, filename);
  return blob;
}

/** 触发浏览器下载 Blob */
export function downloadBlob(blob, filename) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.style.display = 'none';
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  // 释放 URL（让浏览器回收）
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

/** 截 + 序列化为 data URL（用于内嵌预览） */
export async function screenshotDataUrl(el, options = {}) {
  const canvas = await html2canvas(el, { ...DEFAULT_OPTIONS, ...options });
  return canvas.toDataURL('image/png');
}

/** 从 Blob 读取尺寸（用于卡片预览 fallback） */
export function blobToDataUrl(blob) {
  return new Promise((resolve, reject) => {
    const r = new FileReader();
    r.onload = () => resolve(r.result);
    r.onerror = reject;
    r.readAsDataURL(blob);
  });
}
