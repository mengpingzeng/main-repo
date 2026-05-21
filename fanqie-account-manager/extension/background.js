// background.js — 番茄账号管家 Service Worker
// 负责：清除旧登录态 → 新建窗口打开登录页 → 监听 URL 跳转 → 抓取 Cookie → 获取笔名 → 结果回传前端

const TARGET_DOMAINS = [
  '.fanqienovel.com',
  'fanqienovel.com',
  '.snssdk.com',
  '.bytedance.com',
  'passport.bytedance.com',
];

const ORIGINS_TO_CLEAR = [
  'https://fanqienovel.com',
  'https://www.fanqienovel.com',
  'https://sso.snssdk.com',
  'https://passport.bytedance.com',
  'https://snssdk.com',
  'https://bytedance.com',
  'https://accounts.bytedance.com',
  'https://login.bytedance.com',
];

// 登录成功后，番茄会跳转离开登录页；检测到 URL 不再包含 /login 即为登录成功
// （不再用 Cookie 变化检测，避免「获取验证码」时写入的 CSRF Cookie 误触发）
const LOGIN_PAGE_PATTERN = /\/login/i;

// 账号信息接口（在番茄 Tab 内部调用，自动携带 Cookie 和 msToken）
const ACCOUNT_INFO_API = 'https://fanqienovel.com/api/author/account/info/v0/';

// 运行时状态（Service Worker 存活期内有效）
let state = {
  active: false,
  managementTabId: null,
  fanqieTabId: null,
  fanqieWinId: null,
};
let cookieListener = null;
let timeoutHandle = null;
let pollHandle = null; // 轮询句柄，作为 URL 事件监听的兜底

// ─────────────────────────────────────────────
// 消息入口：来自 content.js 的 FANQIE_CAPTURE_START
// ─────────────────────────────────────────────
chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (msg.type === 'FANQIE_CAPTURE_START') {
    handleStartCapture(sender.tab.id);
    sendResponse({ ok: true });
  } else if (msg.type === 'FANQIE_MANUAL_CAPTURE') {
    // 用户在页面点击「我已完成登录」手动触发抓取
    if (state.active) {
      captureCookiesAndFinish();
    }
    sendResponse({ ok: true });
  } else if (msg.type === 'FANQIE_INJECT_COOKIES') {
    // 把 Vault 里存的 Cookie 注入浏览器，然后打开番茄写作者中心
    handleInjectCookies(msg.cookieStr, sender.tab.id);
    sendResponse({ ok: true });
  }
  return true;
});

// ─────────────────────────────────────────────
// Step 1：开始抓取流程
// ─────────────────────────────────────────────
async function handleStartCapture(managementTabId) {
  if (state.active) {
    sendToTab(managementTabId, {
      type: 'FANQIE_CAPTURE_STATUS',
      status: 'busy',
      message: '已有抓取任务进行中，请等待当前流程完成',
    });
    return;
  }

  state = { active: true, managementTabId, fanqieTabId: null, fanqieWinId: null };
  loginDetected = false;

  sendToTab(managementTabId, {
    type: 'FANQIE_CAPTURE_STATUS',
    status: 'clearing',
    message: '正在清除旧登录态...',
  });

  // 清除番茄 + 字节 SSO 全部 Cookie，确保登录环境干净
  // 用两种方式双保险：browsingData.remove（清 localStorage 等）+ cookies.remove（逐条删 Cookie）
  try {
    await chrome.browsingData.remove(
      { origins: ORIGINS_TO_CLEAR },
      { cookies: true, localStorage: true, indexedDB: true, cacheStorage: true }
    );
  } catch (e) {
    console.warn('[FanqieExt] browsingData.remove error:', e);
  }

  // 再用 cookies API 按域名逐条删除，覆盖 browsingData 可能漏掉的跨子域 Cookie
  for (const domain of TARGET_DOMAINS) {
    try {
      const cookies = await chrome.cookies.getAll({ domain });
      for (const c of cookies) {
        const url = `https://${c.domain.replace(/^\./, '')}${c.path}`;
        await chrome.cookies.remove({ url, name: c.name }).catch(() => {});
      }
    } catch (e) {
      console.warn(`[FanqieExt] cookies.remove(${domain}) error:`, e);
    }
  }

  sendToTab(managementTabId, {
    type: 'FANQIE_CAPTURE_STATUS',
    status: 'login_pending',
    message: '请在打开的番茄小说页面完成手机验证码登录...',
  });

  // 新建独立窗口，直接打开番茄创作者登录页
  const win = await chrome.windows.create({
    url: 'https://fanqienovel.com/main/writer/login',
    type: 'normal',
    focused: true,
    width: 1024,
    height: 768,
  });
  state.fanqieTabId = win.tabs?.[0]?.id ?? null;
  state.fanqieWinId = win.id ?? null;

  // 开始监听登录成功（通过 URL 跳转检测，不依赖 Cookie 变化）
  startLoginMonitor();

  // 5 分钟超时保护
  timeoutHandle = setTimeout(() => {
    if (state.active) doCancel('等待登录超时（5 分钟），请重新操作');
  }, 5 * 60 * 1000);
}

// ─────────────────────────────────────────────
// Step 2：监听 Tab URL 变化，检测登录成功
// 登录成功后番茄会将页面从 /login 跳转到写作者中心，
// 这是比 Cookie 变化更可靠的登录完成信号。
// 双重保障：chrome.tabs.onUpdated 事件 + 每 2 秒轮询，防止 SSO 快速跳转漏检。
// ─────────────────────────────────────────────
function startLoginMonitor() {
  cookieListener = (tabId, changeInfo, tab) => {
    if (!state.active) return;
    if (tabId !== state.fanqieTabId) return;

    // 同时兼容两种跳转方式：
    // 1. changeInfo.url 有值 → SPA 用 history.pushState 改了 URL（不触发完整加载）
    // 2. changeInfo.status === 'complete' → 完整页面加载完成
    const url = changeInfo.url || (changeInfo.status === 'complete' ? tab.url : '');
    if (!url) return;

    checkLoginSuccess(url);
  };
  chrome.tabs.onUpdated.addListener(cookieListener);

  // 兜底轮询：每 2 秒主动读一次标签页当前 URL
  // 解决 SSO 自动跳转、重定向速度过快导致事件被漏掉的问题
  pollHandle = setInterval(async () => {
    if (!state.active || !state.fanqieTabId) return;
    try {
      const tab = await chrome.tabs.get(state.fanqieTabId);
      if (tab?.url) checkLoginSuccess(tab.url);
    } catch (_) {
      // 标签页已关闭，忽略
    }
  }, 2000);
}

// checkLoginSuccess 检查 URL 是否已离开登录页，避免重复触发
let loginDetected = false;
function checkLoginSuccess(url) {
  if (!state.active || loginDetected) return;
  const isFanqie = url.includes('fanqienovel.com');
  const isLoginPage = LOGIN_PAGE_PATTERN.test(url);

  if (isFanqie && !isLoginPage) {
    loginDetected = true;
    stopLoginMonitor();

    sendToTab(state.managementTabId, {
      type: 'FANQIE_CAPTURE_STATUS',
      status: 'capturing',
      message: '检测到登录成功，正在收集 Cookie...',
    });

    // 等待 4 秒：让所有 Cookie 落下，并等页面发起 account/info 等接口
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

  // 从所有目标域收集 Cookie，去重
  const seen = new Set();
  const allCookies = [];
  for (const domain of TARGET_DOMAINS) {
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
      console.warn(`[FanqieExt] getAll(${domain}) failed:`, e);
    }
  }

  if (allCookies.length === 0) {
    doCancel('未能获取到 Cookie，请重试');
    return;
  }

  const cookieStr = allCookies.map((c) => `${c.name}=${c.value}`).join('; ');

  // 尝试获取用户昵称（失败不影响主流程）
  let username = null;
  try {
    username = await tryFetchUsername();
  } catch (e) {
    console.warn('[FanqieExt] fetchUsername failed:', e);
  }

  // 关闭番茄窗口，保存 id 后先清状态
  const fanqieTabId = state.fanqieTabId;
  const fanqieWinId = state.fanqieWinId;
  const managementTabId = state.managementTabId;
  state = { active: false, managementTabId: null, fanqieTabId: null, fanqieWinId: null };
  loginDetected = false;

  // 抓取完毕后清除浏览器里的番茄 Cookie，避免与 Vault 存储的 session 产生冲突
  // （若用户在浏览器里退出登录，会使 Vault 里的 Cookie 同步失效）
  for (const domain of TARGET_DOMAINS) {
    try {
      const cookies = await chrome.cookies.getAll({ domain });
      for (const c of cookies) {
        const url = `https://${c.domain.replace(/^\./, '')}${c.path}`;
        await chrome.cookies.remove({ url, name: c.name }).catch(() => {});
      }
    } catch (e) {
      console.warn(`[FanqieExt] post-capture clear(${domain}) error:`, e);
    }
  }

  // 回传结果给管理页面
  sendToTab(managementTabId, {
    type: 'FANQIE_CAPTURE_RESULT',
    cookieStr,
    username,
    cookieCount: allCookies.length,
  });

  if (fanqieWinId) {
    chrome.windows.remove(fanqieWinId).catch(() => {});
  } else if (fanqieTabId) {
    chrome.tabs.remove(fanqieTabId).catch(() => {});
  }
}

// ─────────────────────────────────────────────
// 辅助：获取用户昵称
// ─────────────────────────────────────────────
async function tryFetchUsername() {
  if (!state.fanqieTabId) return null;

  // 在番茄 Tab 内部执行：同源请求自动携带 Cookie；msToken / a_bogus 从页面环境读取
  try {
    const results = await chrome.scripting.executeScript({
      target: { tabId: state.fanqieTabId },
      func: async () => {
        const pickAuthorName = (data) => {
          if (data?.code === 0 && data?.data?.author_name) {
            return data.data.author_name;
          }
          return null;
        };

        const resources = performance.getEntriesByType('resource');

        // 方法一：复用页面已发起的 account/info 请求（含完整 msToken + a_bogus）
        for (const entry of resources) {
          if (!entry.name.includes('/api/author/account/info/v0/')) continue;
          try {
            const resp = await fetch(entry.name, { credentials: 'include' });
            if (!resp.ok) continue;
            const name = pickAuthorName(await resp.json());
            if (name) return name;
          } catch (_) {}
        }

        // 方法二：从近期请求 URL 提取 a_bogus，再主动调用接口
        let aBogus = '';
        for (const entry of resources) {
          if (!entry.name.includes('a_bogus=')) continue;
          const match = entry.name.match(/[?&]a_bogus=([^&]+)/);
          if (match) {
            aBogus = decodeURIComponent(match[1]);
            break;
          }
        }

        try {
          const msToken = localStorage.getItem('xmst') || '';
          const params = new URLSearchParams({
            aid: '2503',
            app_name: 'muye_novel',
          });
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
    console.warn('[FanqieExt] executeScript for username failed:', e);
  }

  return null;
}

// ─────────────────────────────────────────────
// 辅助：取消并通知
// ─────────────────────────────────────────────
function doCancel(reason) {
  if (cookieListener) {
    chrome.tabs.onUpdated.removeListener(cookieListener);
    cookieListener = null;
  }
  if (timeoutHandle) {
    clearTimeout(timeoutHandle);
    timeoutHandle = null;
  }
  if (pollHandle) {
    clearInterval(pollHandle);
    pollHandle = null;
  }
  loginDetected = false;

  const managementTabId = state.managementTabId;
  const fanqieTabId = state.fanqieTabId;
  const fanqieWinId = state.fanqieWinId;
  state = { active: false, managementTabId: null, fanqieTabId: null, fanqieWinId: null };

  if (fanqieWinId) {
    chrome.windows.remove(fanqieWinId).catch(() => {});
  } else if (fanqieTabId) {
    chrome.tabs.remove(fanqieTabId).catch(() => {});
  }
  if (managementTabId) {
    sendToTab(managementTabId, {
      type: 'FANQIE_CAPTURE_ERROR',
      message: reason,
    });
  }
}

function sendToTab(tabId, msg) {
  if (!tabId) return;
  chrome.tabs.sendMessage(tabId, msg).catch((e) => {
    console.warn('[FanqieExt] sendMessage to tab failed:', e);
  });
  // 同步状态到 storage，供 popup 轮询展示
  if (msg.type === 'FANQIE_CAPTURE_STATUS') {
    chrome.storage.local.set({ captureActive: true, captureMessage: msg.message });
  } else if (msg.type === 'FANQIE_CAPTURE_RESULT' || msg.type === 'FANQIE_CAPTURE_ERROR') {
    chrome.storage.local.set({ captureActive: false, captureMessage: '' });
  }
}

// 用户手动关闭了番茄窗口 → 取消流程
chrome.windows.onRemoved.addListener((winId) => {
  if (state.active && winId === state.fanqieWinId) {
    state.fanqieTabId = null;
    state.fanqieWinId = null;
    doCancel('番茄小说窗口被关闭，请重新操作');
  }
});

// ─────────────────────────────────────────────
// Cookie 注入：把 Vault 里存的 Cookie 写入浏览器，然后打开番茄写作者中心
// ─────────────────────────────────────────────

/**
 * parseCookieString 将 "key=val; key2=val2" 格式解析为对象数组。
 * 支持 URL 编码的 value。
 */
function parseCookieString(cookieStr) {
  return cookieStr.split(';').map(s => s.trim()).filter(Boolean).map(seg => {
    const idx = seg.indexOf('=');
    if (idx <= 0) return null;
    const name = seg.slice(0, idx).trim();
    const value = seg.slice(idx + 1).trim();
    return { name, value };
  }).filter(Boolean);
}

async function handleInjectCookies(cookieStr, managementTabId) {
  if (!cookieStr) {
    sendToTab(managementTabId, { type: 'FANQIE_INJECT_ERROR', message: 'Cookie 为空' });
    return;
  }

  sendToTab(managementTabId, { type: 'FANQIE_INJECT_STATUS', message: '正在清除旧登录态...' });

  // 清除番茄现有 Cookie
  for (const domain of TARGET_DOMAINS) {
    try {
      const cookies = await chrome.cookies.getAll({ domain });
      for (const c of cookies) {
        const url = `https://${c.domain.replace(/^\./, '')}${c.path}`;
        await chrome.cookies.remove({ url, name: c.name }).catch(() => {});
      }
    } catch (e) {
      console.warn(`[FanqieExt] inject clear(${domain}) error:`, e);
    }
  }

  sendToTab(managementTabId, { type: 'FANQIE_INJECT_STATUS', message: '正在注入 Cookie...' });

  // 逐条注入 Cookie，写入 fanqienovel.com
  const cookies = parseCookieString(cookieStr);
  let successCount = 0;
  for (const { name, value } of cookies) {
    try {
      await chrome.cookies.set({
        url: 'https://fanqienovel.com',
        name,
        value,
        domain: '.fanqienovel.com',
        path: '/',
        secure: true,
        sameSite: 'lax',
      });
      successCount++;
    } catch (e) {
      // 部分 cookie 可能因 domain 不匹配等原因失败，忽略单条错误
      console.warn(`[FanqieExt] set cookie ${name} failed:`, e);
    }
  }

  if (successCount === 0) {
    sendToTab(managementTabId, { type: 'FANQIE_INJECT_ERROR', message: 'Cookie 注入失败，请重试' });
    return;
  }

  sendToTab(managementTabId, { type: 'FANQIE_INJECT_STATUS', message: `已注入 ${successCount} 条 Cookie，正在打开番茄...` });

  // 打开番茄写作者中心
  chrome.tabs.create({ url: 'https://fanqienovel.com/main/writer/', active: true });

  sendToTab(managementTabId, { type: 'FANQIE_INJECT_DONE' });
}
