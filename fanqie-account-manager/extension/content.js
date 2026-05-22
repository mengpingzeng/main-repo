// content.js — 注入到所有页面，作为「网页 ↔ 扩展 background」的通信桥
// 网页通过 window.postMessage 触发，background 通过 chrome.tabs.sendMessage 回传

// 检查扩展上下文是否仍然有效
function isExtensionAlive() {
  try {
    return !!(chrome?.runtime?.id);
  } catch (_) {
    return false;
  }
}

// 需要转发给 background 的消息类型
const FORWARD_TO_BG = ['FANQIE_CAPTURE_START', 'FANQIE_MANUAL_CAPTURE', 'FANQIE_INJECT_COOKIES'];

// ── 监听来自网页的触发消息 ──────────────────────────
window.addEventListener('message', (event) => {
  if (event.source !== window) return;
  if (!event.data || typeof event.data !== 'object') return;
  if (!FORWARD_TO_BG.includes(event.data.type)) return;

  // 扩展上下文失效（页面未刷新就重装了扩展）
  if (!isExtensionAlive()) {
    window.postMessage({
      type: 'FANQIE_CAPTURE_ERROR',
      message: '扩展上下文已失效，请刷新页面后重试',
    }, '*');
    return;
  }

  try {
    chrome.runtime.sendMessage({ type: event.data.type, cookieStr: event.data.cookieStr, platform: event.data.platform }, (response) => {
      if (chrome.runtime.lastError) {
        window.postMessage({
          type: 'FANQIE_CAPTURE_ERROR',
          message: '未检测到「番茄账号管家」扩展，请安装并启用后重试',
        }, '*');
      }
    });
  } catch (_) {
    window.postMessage({
      type: 'FANQIE_CAPTURE_ERROR',
      message: '扩展通信异常，请刷新页面后重试',
    }, '*');
  }
});

// ── 监听来自 background 的结果，转发给网页 ──────────
if (isExtensionAlive()) {
  chrome.runtime.onMessage.addListener((message) => {
    const relayTypes = [
      'FANQIE_CAPTURE_RESULT',
      'FANQIE_CAPTURE_STATUS',
      'FANQIE_CAPTURE_ERROR',
      'FANQIE_INJECT_STATUS',
      'FANQIE_INJECT_DONE',
      'FANQIE_INJECT_ERROR',
    ];
    if (relayTypes.includes(message.type)) {
      window.postMessage(message, '*');
    }
  });
}
