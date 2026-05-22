// background.js — 账号管家 Service Worker
// 支持平台：fanqie（番茄小说）、zhulang（逐浪网）
// 负责：清除旧登录态 → 新建窗口打开登录页 → 监听 URL 跳转 → 抓取 Cookie → 获取用户名 → 结果回传前端

// ─────────────────────────────────────────────
// 平台配置表
// ─────────────────────────────────────────────
const PLATFORM_CONFIG = {
  fanqie: {
    domains: [
      '.fanqienovel.com',
      'fanqienovel.com',
      '.snssdk.com',
      '.bytedance.com',
      'passport.bytedance.com',
    ],
    clearOrigins: [
      'https://fanqienovel.com',
      'https://www.fanqienovel.com',
      'https://sso.snssdk.com',
      'https://passport.bytedance.com',
      'https://snssdk.com',
      'https://bytedance.com',
      'https://accounts.bytedance.com',
      'https://login.bytedance.com',
    ],
    loginUrl: 'https://fanqienovel.com/main/writer/login',
    writerUrl: 'https://fanqienovel.com/main/writer/',
    injectDomain: '.fanqienovel.com',
    injectUrl: 'https://fanqienovel.com',
    // 判断当前 URL 是否属于该平台（用于登录成功检测）
    isSiteDomain: (url) => url.includes('fanqienovel.com'),
    loginPendingMessage: '请在打开的番茄小说页面完成手机验证码登录...',
  },
  zhulang: {
    domains: [
      '.zhulang.com',
      'www.zhulang.com',
      'writer.zhulang.com',
    ],
    clearOrigins: [
      'https://www.zhulang.com',
      'https://zhulang.com',
      'https://writer.zhulang.com',
    ],
    loginUrl: 'https://www.zhulang.com/login/index.html',
    writerUrl: 'https://writer.zhulang.com/book/index.html',
    injectDomain: '.zhulang.com',
    injectUrl: 'https://www.zhulang.com',
    isSiteDomain: (url) => url.includes('zhulang.com'),
    loginPendingMessage: '请在打开的逐浪网页面完成登录...',
  },
};

// 登录成功后页面会离开 /login 路径
const LOGIN_PAGE_PATTERN = /\/login/i;

// 运行时状态（Service Worker 存活期内有效）
let state = {
  active: false,
  platform: 'fanqie',
  managementTabId: null,
  targetTabId: null,
  targetWinId: null,
};
let cookieListener = null;
let timeoutHandle = null;
let pollHandle = null;

// ─────────────────────────────────────────────
// 消息入口
// ─────────────────────────────────────────────
chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (msg.type === 'FANQIE_CAPTURE_START') {
    handleStartCapture(sender.tab.id, msg.platform || 'fanqie');
    sendResponse({ ok: true });
  } else if (msg.type === 'FANQIE_MANUAL_CAPTURE') {
    if (state.active) captureCookiesAndFinish();
    sendResponse({ ok: true });
  } else if (msg.type === 'FANQIE_INJECT_COOKIES') {
    handleInjectCookies(msg.cookieStr, msg.platform || 'fanqie', sender.tab.id);
    sendResponse({ ok: true });
  }
  return true;
});

// ─────────────────────────────────────────────
// Step 1：开始抓取流程
// ─────────────────────────────────────────────
async function handleStartCapture(managementTabId, platform) {
  const cfg = PLATFORM_CONFIG[platform] || PLATFORM_CONFIG.fanqie;

  if (state.active) {
    sendToTab(managementTabId, {
      type: 'FANQIE_CAPTURE_STATUS',
      status: 'busy',
      message: '已有抓取任务进行中，请等待当前流程完成',
    });
    return;
  }

  state = { active: true, platform, managementTabId, targetTabId: null, targetWinId: null };
  loginDetected = false;

  sendToTab(managementTabId, {
    type: 'FANQIE_CAPTURE_STATUS',
    status: 'clearing',
    message: '正在清除旧登录态...',
  });

  // browsingData.remove 清除 localStorage 等
  try {
    await chrome.browsingData.remove(
      { origins: cfg.clearOrigins },
      { cookies: true, localStorage: true, indexedDB: true, cacheStorage: true }
    );
  } catch (e) {
    console.warn('[Ext] browsingData.remove error:', e);
  }

  // cookies API 逐条删除，覆盖跨子域漏删的情况
  for (const domain of cfg.domains) {
    try {
      const cookies = await chrome.cookies.getAll({ domain });
      for (const c of cookies) {
        const url = `https://${c.domain.replace(/^\./, '')}${c.path}`;
        await chrome.cookies.remove({ url, name: c.name }).catch(() => {});
      }
    } catch (e) {
      console.warn(`[Ext] cookies.remove(${domain}) error:`, e);
    }
  }

  sendToTab(managementTabId, {
    type: 'FANQIE_CAPTURE_STATUS',
    status: 'login_pending',
    message: cfg.loginPendingMessage,
  });

  const win = await chrome.windows.create({
    url: cfg.loginUrl,
    type: 'normal',
    focused: true,
    width: 1024,
    height: 768,
  });
  state.targetTabId = win.tabs?.[0]?.id ?? null;
  state.targetWinId = win.id ?? null;

  startLoginMonitor();

  timeoutHandle = setTimeout(() => {
    if (state.active) doCancel('等待登录超时（5 分钟），请重新操作');
  }, 5 * 60 * 1000);
}

// ─────────────────────────────────────────────
// Step 2：监听 Tab URL 变化，检测登录成功
// 双重保障：chrome.tabs.onUpdated 事件 + 每 2 秒轮询
// ─────────────────────────────────────────────
function startLoginMonitor() {
  cookieListener = (tabId, changeInfo, tab) => {
    if (!state.active) return;
    if (tabId !== state.targetTabId) return;
    const url = changeInfo.url || (changeInfo.status === 'complete' ? tab.url : '');
    if (!url) return;
    checkLoginSuccess(url);
  };
  chrome.tabs.onUpdated.addListener(cookieListener);

  pollHandle = setInterval(async () => {
    if (!state.active || !state.targetTabId) return;
    try {
      const tab = await chrome.tabs.get(state.targetTabId);
      if (tab?.url) checkLoginSuccess(tab.url);
    } catch (_) {}
  }, 2000);
}

let loginDetected = false;
function checkLoginSuccess(url) {
  if (!state.active || loginDetected) return;
  const cfg = PLATFORM_CONFIG[state.platform] || PLATFORM_CONFIG.fanqie;
  const isSite = cfg.isSiteDomain(url);
  const isLoginPage = LOGIN_PAGE_PATTERN.test(url);

  if (isSite && !isLoginPage) {
    loginDetected = true;
    stopLoginMonitor();

    sendToTab(state.managementTabId, {
      type: 'FANQIE_CAPTURE_STATUS',
      status: 'capturing',
      message: '检测到登录成功，正在收集 Cookie...',
    });

    setTimeout(captureCookiesAndFinish, 4000);
  }
}

function stopLoginMonitor() {
  if (cookieListener) {
    chrome.tabs.onUpdated.removeListener(cookieListener);
    cookieListener = null;
  }
  if (pollHandle) {
    clearInterval(pollHandle);
    pollHandle = null;
  }
}

// ─────────────────────────────────────────────
// Step 3：收集 Cookie + 尝试获取用户名 + 回传结果
// ─────────────────────────────────────────────
async function captureCookiesAndFinish() {
  if (!state.active) return;

  if (timeoutHandle) {
    clearTimeout(timeoutHandle);
    timeoutHandle = null;
  }

  const cfg = PLATFORM_CONFIG[state.platform] || PLATFORM_CONFIG.fanqie;

  // 收集所有目标域 Cookie，去重
  const seen = new Set();
  const allCookies = [];
  for (const domain of cfg.domains) {
    try {
      const cookies = await chrome.cookies.getAll({ domain });
      for (const c of cookies) {
        const key = `${c.name}@${c.domain}`;
        if (!seen.has(key)) {
          seen.add(key);
          allCookies.push(c);
        }
      }
    } catch (e) {
      console.warn(`[Ext] getAll(${domain}) failed:`, e);
    }
  }

  if (allCookies.length === 0) {
    doCancel('未能获取到 Cookie，请重试');
    return;
  }

  const cookieStr = allCookies.map((c) => `${c.name}=${c.value}`).join('; ');

  // 尝试获取用户昵称（番茄和逐浪均支持）
  let username = null;
  if (state.platform === 'fanqie') {
    try {
      username = await tryFetchFanqieUsername();
    } catch (e) {
      console.warn('[Ext] fetchUsername(fanqie) failed:', e);
    }
  } else if (state.platform === 'zhulang') {
    try {
      username = await tryFetchZhulangUsername();
    } catch (e) {
      console.warn('[Ext] fetchUsername(zhulang) failed:', e);
    }
  }

  const targetTabId = state.targetTabId;
  const targetWinId = state.targetWinId;
  const managementTabId = state.managementTabId;
  state = { active: false, platform: 'fanqie', managementTabId: null, targetTabId: null, targetWinId: null };
  loginDetected = false;

  // 抓取完毕后清除浏览器里的平台 Cookie，避免 Vault session 被浏览器操作意外失效
  for (const domain of cfg.domains) {
    try {
      const cookies = await chrome.cookies.getAll({ domain });
      for (const c of cookies) {
        const url = `https://${c.domain.replace(/^\./, '')}${c.path}`;
        await chrome.cookies.remove({ url, name: c.name }).catch(() => {});
      }
    } catch (e) {
      console.warn(`[Ext] post-capture clear(${domain}) error:`, e);
    }
  }

  sendToTab(managementTabId, {
    type: 'FANQIE_CAPTURE_RESULT',
    cookieStr,
    username,
    cookieCount: allCookies.length,
  });

  if (targetWinId) {
    chrome.windows.remove(targetWinId).catch(() => {});
  } else if (targetTabId) {
    chrome.tabs.remove(targetTabId).catch(() => {});
  }
}

// ─────────────────────────────────────────────
// 辅助：获取番茄用户昵称（平台专属）
// ─────────────────────────────────────────────
async function tryFetchFanqieUsername() {
  if (!state.targetTabId) return null;

  try {
    const results = await chrome.scripting.executeScript({
      target: { tabId: state.targetTabId },
      func: async () => {
        const pickAuthorName = (data) => {
          if (data?.code === 0 && data?.data?.author_name) return data.data.author_name;
          return null;
        };
        const resources = performance.getEntriesByType('resource');

        for (const entry of resources) {
          if (!entry.name.includes('/api/author/account/info/v0/')) continue;
          try {
            const resp = await fetch(entry.name, { credentials: 'include' });
            if (!resp.ok) continue;
            const name = pickAuthorName(await resp.json());
            if (name) return name;
          } catch (_) {}
        }

        let aBogus = '';
        for (const entry of resources) {
          if (!entry.name.includes('a_bogus=')) continue;
          const match = entry.name.match(/[?&]a_bogus=([^&]+)/);
          if (match) { aBogus = decodeURIComponent(match[1]); break; }
        }

        try {
          const msToken = localStorage.getItem('xmst') || '';
          const params = new URLSearchParams({ aid: '2503', app_name: 'muye_novel' });
          if (msToken) params.set('msToken', msToken);
          if (aBogus) params.set('a_bogus', aBogus);
          const resp = await fetch(
            `https://fanqienovel.com/api/author/account/info/v0/?${params.toString()}`,
            { credentials: 'include' }
          );
          if (!resp.ok) return null;
          return pickAuthorName(await resp.json());
        } catch (_) {}

        return null;
      },
    });
    const name = results?.[0]?.result;
    if (name) return name;
  } catch (e) {
    console.warn('[Ext] executeScript for username failed:', e);
  }

  return null;
}

// ─────────────────────────────────────────────
// 辅助：获取逐浪作者名（平台专属）
// 登录后跳转页（www.zhulang.com）只有用户名，作者名在写作中心。
// 策略：把登录 tab 导航到写作中心，等页面加载后提取 li.uinfo > em。
// ─────────────────────────────────────────────
async function tryFetchZhulangUsername() {
  if (!state.targetTabId) return null;

  try {
    // 导航到写作中心（此时 cookie 已落下，能正常访问）
    await chrome.tabs.update(state.targetTabId, {
      url: 'https://writer.zhulang.com/book/index.html',
    });

    // 等待页面加载完成（最多 8 秒）
    await new Promise((resolve) => {
      const listener = (tabId, changeInfo) => {
        if (tabId === state.targetTabId && changeInfo.status === 'complete') {
          chrome.tabs.onUpdated.removeListener(listener);
          resolve();
        }
      };
      chrome.tabs.onUpdated.addListener(listener);
      setTimeout(() => {
        chrome.tabs.onUpdated.removeListener(listener);
        resolve();
      }, 8000);
    });

    // 额外等 1 秒，让 Vue 等框架完成渲染
    await new Promise(r => setTimeout(r, 1000));

    const results = await chrome.scripting.executeScript({
      target: { tabId: state.targetTabId },
      func: () => {
        // li.uinfo > em 是逐浪写作中心导航栏的作者名节点
        const el = document.querySelector('li.uinfo em') ||
                   document.querySelector('.uinfo em');
        const name = el?.textContent?.trim();
        if (name && name.length > 0 && name.length < 30) return name;
        return null;
      },
    });

    return results?.[0]?.result || null;
  } catch (e) {
    console.warn('[Ext] fetchZhulangUsername failed:', e);
    return null;
  }
}

// ─────────────────────────────────────────────
// 辅助：取消并通知
// ─────────────────────────────────────────────
function doCancel(reason) {
  stopLoginMonitor();
  if (timeoutHandle) { clearTimeout(timeoutHandle); timeoutHandle = null; }
  loginDetected = false;

  const managementTabId = state.managementTabId;
  const targetTabId = state.targetTabId;
  const targetWinId = state.targetWinId;
  state = { active: false, platform: 'fanqie', managementTabId: null, targetTabId: null, targetWinId: null };

  if (targetWinId) {
    chrome.windows.remove(targetWinId).catch(() => {});
  } else if (targetTabId) {
    chrome.tabs.remove(targetTabId).catch(() => {});
  }
  if (managementTabId) {
    sendToTab(managementTabId, { type: 'FANQIE_CAPTURE_ERROR', message: reason });
  }
}

function sendToTab(tabId, msg) {
  if (!tabId) return;
  chrome.tabs.sendMessage(tabId, msg).catch((e) => {
    console.warn('[Ext] sendMessage to tab failed:', e);
  });
  if (msg.type === 'FANQIE_CAPTURE_STATUS') {
    chrome.storage.local.set({ captureActive: true, captureMessage: msg.message });
  } else if (msg.type === 'FANQIE_CAPTURE_RESULT' || msg.type === 'FANQIE_CAPTURE_ERROR') {
    chrome.storage.local.set({ captureActive: false, captureMessage: '' });
  }
}

// 用户手动关闭了登录窗口 → 取消流程
chrome.windows.onRemoved.addListener((winId) => {
  if (state.active && winId === state.targetWinId) {
    state.targetTabId = null;
    state.targetWinId = null;
    doCancel('登录窗口被关闭，请重新操作');
  }
});

// ─────────────────────────────────────────────
// Cookie 注入：把 Vault 里存的 Cookie 写入浏览器，然后打开目标平台
// ─────────────────────────────────────────────
function parseCookieString(cookieStr) {
  return cookieStr.split(';').map(s => s.trim()).filter(Boolean).map(seg => {
    const idx = seg.indexOf('=');
    if (idx <= 0) return null;
    return { name: seg.slice(0, idx).trim(), value: seg.slice(idx + 1).trim() };
  }).filter(Boolean);
}

async function handleInjectCookies(cookieStr, platform, managementTabId) {
  const cfg = PLATFORM_CONFIG[platform] || PLATFORM_CONFIG.fanqie;

  if (!cookieStr) {
    sendToTab(managementTabId, { type: 'FANQIE_INJECT_ERROR', message: 'Cookie 为空' });
    return;
  }

  sendToTab(managementTabId, { type: 'FANQIE_INJECT_STATUS', message: '正在清除旧登录态...' });

  for (const domain of cfg.domains) {
    try {
      const cookies = await chrome.cookies.getAll({ domain });
      for (const c of cookies) {
        const url = `https://${c.domain.replace(/^\./, '')}${c.path}`;
        await chrome.cookies.remove({ url, name: c.name }).catch(() => {});
      }
    } catch (e) {
      console.warn(`[Ext] inject clear(${domain}) error:`, e);
    }
  }

  sendToTab(managementTabId, { type: 'FANQIE_INJECT_STATUS', message: '正在注入 Cookie...' });

  const cookies = parseCookieString(cookieStr);
  let successCount = 0;
  for (const { name, value } of cookies) {
    try {
      await chrome.cookies.set({
        url: cfg.injectUrl,
        name,
        value,
        domain: cfg.injectDomain,
        path: '/',
        secure: true,
        sameSite: 'lax',
      });
      successCount++;
    } catch (e) {
      console.warn(`[Ext] set cookie ${name} failed:`, e);
    }
  }

  if (successCount === 0) {
    sendToTab(managementTabId, { type: 'FANQIE_INJECT_ERROR', message: 'Cookie 注入失败，请重试' });
    return;
  }

  sendToTab(managementTabId, { type: 'FANQIE_INJECT_STATUS', message: `已注入 ${successCount} 条 Cookie，正在打开页面...` });
  chrome.tabs.create({ url: cfg.writerUrl, active: true });
  sendToTab(managementTabId, { type: 'FANQIE_INJECT_DONE' });
}
