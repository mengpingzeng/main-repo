const DEFAULT_SERVER_URL = 'http://47.107.124.45:3000';

const serverUrlInput = document.getElementById('serverUrl');
const saveBtn = document.getElementById('saveBtn');
const openBtn = document.getElementById('openBtn');
const saveTip = document.getElementById('saveTip');
const statusBar = document.getElementById('statusBar');
const statusDot = document.getElementById('statusDot');
const statusText = document.getElementById('statusText');

// 加载已保存的服务器地址
chrome.storage.local.get(['serverUrl'], (result) => {
  serverUrlInput.value = result.serverUrl || DEFAULT_SERVER_URL;
});

// 保存服务器地址
saveBtn.addEventListener('click', () => {
  const url = serverUrlInput.value.trim() || DEFAULT_SERVER_URL;
  chrome.storage.local.set({ serverUrl: url }, () => {
    saveTip.textContent = '✓ 已保存';
    setTimeout(() => { saveTip.textContent = ''; }, 2000);
  });
});

// 打开管理页面
openBtn.addEventListener('click', () => {
  const base = serverUrlInput.value.trim() || DEFAULT_SERVER_URL;
  const url = base.replace(/\/$/, '') + '/accounts';
  chrome.tabs.create({ url });
  window.close();
});

// 轮询抓取状态（每秒刷新一次，Service Worker 存活时有效）
function refreshStatus() {
  chrome.storage.local.get(['captureActive', 'captureMessage'], (result) => {
    if (result.captureActive) {
      statusBar.className = 'status-bar status-active';
      statusDot.className = 'status-dot dot-active';
      statusText.textContent = result.captureMessage || '正在抓取中...';
    } else {
      statusBar.className = 'status-bar status-idle';
      statusDot.className = 'status-dot dot-idle';
      statusText.textContent = '就绪';
    }
  });
}

refreshStatus();
const timer = setInterval(refreshStatus, 1000);
window.addEventListener('unload', () => clearInterval(timer));
