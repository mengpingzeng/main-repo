/**
 * fetch_fanqie_stats.js — 番茄小说作品数据中心抓取脚本 (Puppeteer 浏览器自动化)
 *
 * 功能：从番茄小说作家后台的数据中心页面抓取作品数据。
 *
 * 入参（两种模式）：
 *   直接模式：node fetch_fanqie_stats.js '<JSON字符串>'
 *   base64 模式：node fetch_fanqie_stats.js --base64 （从 stdin 读取 base64 编码的 JSON）
 *
 * Cookie 从环境变量 FANQIE_COOKIE 读取（不会打印到日志）。
 *
 * 输出（stdout）：JSON 一行
 *   {"success":true,"post_id":"...","views":123,"likes":45,"comments":7,"shares":3,"current_readers":56}
 *   {"success":false,"error":"原因说明"}
 *
 * 运行环境：Linux 服务器，headless 模式，--no-sandbox
 * 依赖：npm install puppeteer
 */

'use strict';

const puppeteer = require('puppeteer');

// ======================== 日志 ========================

function log(level, msg, data) {
    const entry = { time: new Date().toISOString(), level, msg };
    if (data && typeof data === 'object') {
        for (const key of Object.keys(data)) {
            entry[key] = data[key];
        }
    }
    process.stderr.write(JSON.stringify(entry) + '\n');
}

// ======================== 配置 ========================

const CONFIG = {
    BASE_URL: 'https://fanqienovel.com',
    WRITER_URL: 'https://fanqienovel.com/main/writer/',
    TIMEOUT_MS: 60000,
    NAVIGATION_TIMEOUT: 30000,
    ELEMENT_TIMEOUT: 15000,
    VIEWPORT: { width: 1920, height: 1080 },
};

// ======================== 输出 ========================

function output(result) {
    process.stdout.write(JSON.stringify(result) + '\n');
    process.exit(result.success ? 0 : 1);
}

function fail(errMsg) {
    log('error', 'fetch stats failed', { error: errMsg });
    output({ success: false, error: errMsg });
}

// ======================== 工具函数 ========================

function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

function readStdinBase64() {
    return new Promise((resolve, reject) => {
        const chunks = [];
        process.stdin.setEncoding('utf-8');
        process.stdin.on('data', (chunk) => chunks.push(chunk));
        process.stdin.on('end', () => {
            const raw = chunks.join('').trim();
            if (!raw) { fail('stdin empty: expected base64-encoded JSON'); return; }
            try {
                const json = Buffer.from(raw, 'base64').toString('utf-8');
                resolve(JSON.parse(json));
            } catch (e) {
                fail('base64 decode or JSON parse failed: ' + e.message);
            }
        });
        process.stdin.on('error', (e) => fail('stdin read error: ' + e.message));
    });
}

// ======================== Cookie ========================

function parseCookieString(cookieStr, domain) {
    const domainClean = domain.replace(/^https?:\/\//, '').split('/')[0];
    return cookieStr.split(';').map(pair => {
        const [name, ...rest] = pair.trim().split('=');
        return {
            name: name.trim(),
            value: rest.join('=').trim(),
            domain: '.' + domainClean,
            path: '/',
            httpOnly: false,
            secure: true,
            sameSite: 'Lax',
        };
    }).filter(c => c.name && c.value);
}

// ======================== 主流程 ========================

async function main() {
    const isBase64 = process.argv.includes('--base64');
    let input;

    if (isBase64) {
        input = await readStdinBase64();
    } else {
        try {
            const raw = process.argv[2];
            if (!raw) { fail('missing argument: JSON input or --base64 flag required'); return; }
            input = JSON.parse(raw);
        } catch (e) {
            fail('invalid JSON input: ' + e.message);
            return;
        }
    }

    const { novelName } = input;
    if (!novelName) { fail('missing required field: novelName'); return; }

    const cookieStr = process.env.FANQIE_COOKIE;
    if (!cookieStr || cookieStr.trim() === '') { fail('FANQIE_COOKIE not set'); return; }
    log('info', 'cookie loaded', { cookieLen: cookieStr.length });

    log('info', 'fetching stats', { novelName });

    const browser = await puppeteer.launch({
        headless: 'new',
        args: [
            '--no-sandbox',
            '--disable-setuid-sandbox',
            '--disable-dev-shm-usage',
            '--disable-gpu',
            '--disable-extensions',
            '--disable-background-networking',
            '--disable-sync',
            '--no-first-run',
        ],
    });

    let page;
    try {
        page = await browser.newPage();
        await page.setViewport(CONFIG.VIEWPORT);
        page.setDefaultNavigationTimeout(CONFIG.NAVIGATION_TIMEOUT);
        page.setDefaultTimeout(CONFIG.ELEMENT_TIMEOUT);

        // === 1. 注入 Cookie 并打开作家专区 ===
        log('info', 'setting cookies and navigating to writer');
        const cookies = parseCookieString(cookieStr, CONFIG.BASE_URL);
        await page.setCookie(...cookies);
        await page.goto(CONFIG.WRITER_URL, { waitUntil: 'networkidle2', timeout: CONFIG.NAVIGATION_TIMEOUT });
        await sleep(2000);

        const loggedIn = await checkLogin(page);
        if (!loggedIn) {
            throw new Error('login failed: cookie may be expired');
        }
        log('info', 'login confirmed');

        // === 2. 点击左侧导航"数据中心" ===
        log('info', 'clicking 数据中心 in left nav');
        await clickButton(page, ['数据中心', '数据']);
        await sleep(2000);

        // === 3. 点击"小说数据"子标签 ===
        log('info', 'clicking 小说数据 sub-tab');
        await clickButton(page, ['小说数据', '小说']);
        await sleep(3000);

        // === 4. 检查并切换作品 ===
        log('info', 'checking current novel against target', { novelName });
        await ensureNovelSelected(page, novelName);
        await sleep(2000);

        // === 5. 抓取数据指标 ===
        log('info', 'scraping stats from 小说数据 cards');
        const stats = await scrapeStatsCards(page, novelName);

        log('info', 'stats scraped', {
            views: stats.views, likes: stats.likes,
            comments: stats.comments, shares: stats.shares,
            current_readers: stats.current_readers,
        });
        output({ success: true, ...stats });

    } catch (e) {
        log('error', 'fetch stats error', { error: e.message });
        output({ success: false, error: e.message });
    } finally {
        await browser.close().catch(() => {});
        log('info', 'browser closed');
    }
}

// ======================== 页面操作 ========================

async function checkLogin(page) {
    try {
        await page.waitForFunction(() => {
            return document.body.innerText.includes('工作台');
        }, { timeout: 15000 });
        return true;
    } catch {
        return false;
    }
}

async function clickButton(page, text) {
    const texts = Array.isArray(text) ? text : [text];
    for (let i = 0; i < texts.length; i++) {
        const targetText = texts[i];
        const clicked = await page.evaluate((t) => {
            const candidates = document.querySelectorAll('button, a, span, div[class*="btn"], div[class*="button"]');
            for (const el of candidates) {
                const elText = (el.textContent || '').trim();
                if (elText === t) { el.click(); return true; }
            }
            for (const el of candidates) {
                const elText = (el.textContent || '').trim();
                if (elText.includes(t) && elText.length <= t.length + 4) {
                    el.click(); return true;
                }
            }
            return false;
        }, targetText);
        if (clicked) { log('info', 'button clicked', { text: targetText }); return; }
    }
    throw new Error('button not found: ' + JSON.stringify(texts));
}

/**
 * 确保「小说数据」页面左侧选中的作品是目标 novelName。
 * 如果不是，点击「切换作品」在弹窗中找到目标并切换。
 */
async function ensureNovelSelected(page, novelName) {
    // 检查左侧当前作品名
    const current = await page.evaluate(() => {
        const candidates = document.querySelectorAll([
            'span[class*="novel"]', 'span[class*="book"]', 'span[class*="work"]',
            'div[class*="sidebar"] span', 'div[class*="aside"] span',
            'div[class*="current"]', 'div[class*="selected"] span',
        ].join(','));
        for (const el of candidates) {
            const t = (el.textContent || '').trim();
            if (t.length > 0 && t.length < 50) return t;
        }
        return '';
    });
    log('info', 'current novel in sidebar', { current });

    if (current === novelName) {
        log('info', 'already on target novel');
        return;
    }

    // 需要切换
    log('info', 'switching novel to', { novelName });
    await clickButton(page, ['切换作品', '切换', '切换书籍']);
    await sleep(1500);

    // 在弹窗 / 下拉列表中找目标作品
    const switched = await page.evaluate((name) => {
        // 遍历所有可见元素，找包含 novelName 的可点击条目
        const all = document.querySelectorAll('li, div[class*="item"], div[class*="option"], '
            + 'span[class*="item"], span[class*="option"], a, button, '
            + 'div[class*="row"], div[class*="book"], div[class*="novel"], '
            + '[class*="dropdown"] div, [class*="popup"] div, [class*="modal"] div');
        for (const el of all) {
            const t = (el.textContent || '').trim();
            if (t === name) {
                el.click();
                return true;
            }
        }
        // 模糊匹配
        for (const el of all) {
            const t = (el.textContent || '').trim();
            if (t.includes(name) && t.length <= name.length + 10) {
                el.click();
                return true;
            }
        }
        return false;
    }, novelName);

    if (!switched) {
        throw new Error('cannot find novel "' + novelName + '" in switch popup');
    }

    log('info', 'novel switched');
    await sleep(2000);
}

/**
 * 在"小说数据"页面抓取数据卡片上的指标。
 */
async function scrapeStatsCards(page, novelName) {
    await sleep(2000);

    const stats = await page.evaluate(() => {
        const result = { views: 0, likes: 0, comments: 0, shares: 0, current_readers: 0 };

        // 查找所有数据卡片容器
        const cards = document.querySelectorAll([
            '[class*="data-card"]', '[class*="DataCard"]',
            '[class*="stat-card"]', '[class*="StatCard"]',
            '[class*="metric-card"]', '[class*="MetricCard"]',
            '[class*="overview-card"]', '[class*="OverviewCard"]',
            'div[class*="card"] span[class*="number"]',
            'div[class*="card"] span[class*="value"]',
            'div[class*="card"] div[class*="num"]',
        ].join(','));

        const cardData = [];
        for (const card of cards) {
            const parent = card.closest('div[class*="card"], div[class*="item"], '
                + 'div[class*="stat"], div[class*="metric"], div[class*="col"]') || card;
            const text = (parent.textContent || '').trim();
            if (text.length > 0 && text.length < 100) {
                cardData.push(text);
            }
        }

        // 如果卡片容器没找到，降级用全页文本
        const bodyText = cardData.length > 0 ? cardData.join('\n') : (document.body.innerText || '');

        function extract(label) {
            const re = new RegExp(label + '[\\\\s\\\\n]*[:\\\\uff1a]?[\\\\s\\\\n]*([\\\\d.]+)(万|亿)?', 'i');
            const m = bodyText.match(re);
            if (!m) return 0;
            let num = parseFloat(m[1]);
            if (m[2] === '万') num *= 10000;
            if (m[2] === '亿') num *= 100000000;
            return Math.round(num);
        }

        result.views           = extract('阅读人数');
        result.likes           = extract('加书架人数');
        result.shares          = extract('催更人数');
        result.comments        = extract('评论次数');
        result.current_readers = extract('在读人数');

        // 兜底：如果核心字段为 0，逐个从全页匹配
        if (result.views === 0) {
            const allText = document.body.innerText || '';
            function extractAll(label) {
                const re = new RegExp(label + '[\\\\s\\\\n]*[:\\\\uff1a]?[\\\\s\\\\n]*([\\\\d.]+)(万|亿)?', 'i');
                const m = allText.match(re);
                if (!m) return 0;
                let num = parseFloat(m[1]);
                if (m[2] === '万') num *= 10000;
                if (m[2] === '亿') num *= 100000000;
                return Math.round(num);
            }
            if (result.views === 0)           result.views           = extractAll('阅读人数');
            if (result.likes === 0)           result.likes           = extractAll('加书架人数');
            if (result.shares === 0)          result.shares          = extractAll('催更人数');
            if (result.comments === 0)        result.comments        = extractAll('评论次数');
            if (result.current_readers === 0) result.current_readers = extractAll('在读人数');
        }

        return result;
    });

    return { post_id: novelName, ...stats };
}

// ======================== 启动 ========================

main().catch(err => {
    log('error', 'unhandled error', { error: err.message, stack: err.stack });
    output({ success: false, error: err.message });
});
