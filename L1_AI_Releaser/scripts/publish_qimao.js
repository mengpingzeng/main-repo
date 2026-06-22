/**
 * publish_qimao.js — 七猫小说章节发布脚本 (Puppeteer 浏览器自动化 + fetch API)
 *
 * 运行环境：Linux 服务器，无显示器，无图形界面
 * 启动参数：headless + --no-sandbox --disable-setuid-sandbox --disable-dev-shm-usage
 *
 * 入参：node publish_qimao.js --base64（从 stdin 读取 base64 编码的 JSON）
 *
 * Cookie 从环境变量 QIMAO_COOKIE 读取
 *
 * 输出（stdout）：JSON 一行
 *   {"success":true,"action":"xxx","bookId":"xxx"}
 *   {"success":false,"error":"error description"}
 *
 * 依赖：npm install puppeteer
 */

'use strict';

const puppeteer = require('puppeteer');
const fs = require('fs');

// ======================== 日志 ========================

const LOG_DIR = '/tmp/logs';
try { fs.mkdirSync(LOG_DIR, { recursive: true }); } catch (_) {}

let _logPrefix = '';

function setLogPrefix(novelName) {
    _logPrefix = 'qimao_' + (novelName || 'unknown').replace(/[^a-zA-Z0-9\u4e00-\u9fff_-]/g, '_').substring(0, 60);
}

function log(level, msg, data) {
    const entry = { time: new Date().toISOString(), level, msg, ...(data ? { data } : {}) };
    const line = JSON.stringify(entry) + '\n';
    process.stderr.write(line);
    try {
        fs.appendFileSync(LOG_DIR + '/qimao_publish.log', line);
        if (_logPrefix) { fs.appendFileSync(LOG_DIR + '/' + _logPrefix + '.log', line); }
    } catch (_) {}
}

// ======================== 配置 ========================

const CONFIG = {
    BASE_URL: 'https://zuozhe.qimao.com',
    BOOK_MANAGE_URL: 'https://zuozhe.qimao.com/front/book-manage',
    TIMEOUT_MS: 600000,
    NAVIGATION_TIMEOUT: 60000,
    MAX_RETRIES: 1,
    VIEWPORT: { width: 1920, height: 1080 },
    MIN_CONTENT_LENGTH: 1000,
    DRAFT_WAIT_MS: 3000,
};

let _globalBrowser = null;

async function launchBrowser() {
    log('info', 'launching browser');
    const browser = await puppeteer.launch({
        headless: 'new',
        protocolTimeout: 180000,
        args: [
            '--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage',
            '--disable-gpu', '--disable-extensions', '--disable-background-networking',
            '--disable-sync', '--no-first-run', '--disable-features=TranslateUI',
            '--disable-software-rasterizer', '--memory-pressure-off',
            '--js-flags=--max-old-space-size=256',
        ],
    });
    _globalBrowser = browser;
    return browser;
}

function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

// ======================== stdin 解析 ========================

function readStdinBase64() {
    return new Promise((resolve, reject) => {
        const chunks = [];
        process.stdin.setEncoding('utf-8');
        process.stdin.on('data', (chunk) => chunks.push(chunk));
        process.stdin.on('end', () => {
            const raw = chunks.join('').trim();
            if (!raw) {
                fail('stdin empty: expected base64-encoded JSON');
                return;
            }
            try {
                const json = Buffer.from(raw, 'base64').toString('utf-8');
                const input = JSON.parse(json);
                resolve(input);
            } catch (e) {
                fail('base64 decode or JSON parse failed: ' + e.message);
            }
        });
        process.stdin.on('error', (e) => {
            fail('stdin read error: ' + e.message);
        });
    });
}

function output(result) {
    process.stdout.write(JSON.stringify(result) + '\n');
    process.exit(result.success ? 0 : 1);
}

function fail(errMsg) {
    log('error', 'publish failed', { error: errMsg });
    output({ success: false, error: errMsg });
}

// ======================== Cookie / 登录 ========================

function parseCookieString(cookieStr, domain) {
    const cookies = [];
    const pairs = cookieStr.split(';');
    for (const pair of pairs) {
        const eqIdx = pair.indexOf('=');
        if (eqIdx <= 0) continue;
        const name = pair.substring(0, eqIdx).trim();
        const value = pair.substring(eqIdx + 1).trim();
        if (!name) continue;
        cookies.push({
            name,
            value,
            domain: domain.replace('https://', '').replace('http://', ''),
            path: '/',
            httpOnly: false,
            secure: true,
        });
    }
    return cookies;
}

async function loginAndNavigate(page, cookieStr) {
    log('info', 'setting cookies and navigating to book-manage');
    const domain = 'zuozhe.qimao.com';
    // 先访问一次让域名生效
    await page.goto(CONFIG.BOOK_MANAGE_URL, { waitUntil: 'domcontentloaded', timeout: CONFIG.NAVIGATION_TIMEOUT });
    const cookies = parseCookieString(cookieStr, CONFIG.BASE_URL);
    await page.setCookie(...cookies);
    await page.goto(CONFIG.BOOK_MANAGE_URL, { waitUntil: 'domcontentloaded', timeout: CONFIG.NAVIGATION_TIMEOUT });
    await sleep(5000);

    const loggedIn = await checkLogin(page);
    if (!loggedIn) {
        throw new Error('login failed: cookie may be expired, page redirected to login');
    }
    log('info', 'login confirmed');
}

async function checkLogin(page) {
    try {
        const url = page.url();
        // 如果被重定向到登录页，说明 cookie 失效
        if (url.includes('/register-login') || url.includes('/login')) {
            log('warn', 'page redirected to login', { url });
            return false;
        }
        // 尝试检查页面是否有 "登录" 按钮，有则未登录
        const hasLoginBtn = await page.evaluate(() => {
            const btns = document.querySelectorAll('button, a');
            for (const btn of btns) {
                if ((btn.textContent || '').includes('登录')) return true;
            }
            return false;
        });
        if (hasLoginBtn) {
            log('warn', 'login button found on page, not logged in');
            return false;
        }
        return true;
    } catch (e) {
        log('warn', 'check login error', { error: e.message });
        return false;
    }
}

// ======================== 工具函数 ========================

/**
 * 在浏览器上下文中执行 fetch 并返回结果。
 */
async function browserFetch(page, url, options = {}) {
    const method = options.method || 'GET';
    const headers = options.headers || {};
    const body = options.body || undefined;

    const result = await page.evaluate(async ({ url, method, headers, body }) => {
        const fetchOptions = { method, headers, credentials: 'include' };
        if (body && method !== 'GET') {
            fetchOptions.body = body;
            if (!headers['Content-Type']) {
                fetchOptions.headers['Content-Type'] = 'application/x-www-form-urlencoded';
            }
        }
        try {
            const resp = await fetch(url, fetchOptions);
            const text = await resp.text();
            let json;
            try { json = JSON.parse(text); } catch (_) { json = null; }
            return { status: resp.status, text, json };
        } catch (e) {
            return { status: 0, text: '', json: null, error: e.message };
        }
    }, { url, method, headers, body });

    return result;
}

/**
 * 构建 URLSearchParams 格式的 body
 */
function buildFormBody(params) {
    const form = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) {
        form.append(k, String(v));
    }
    return form.toString();
}

/**
 * 将纯文本转为 HTML 段落
 */
function textToHtml(text) {
    const paragraphs = text.split('\n\n');
    let html = '';
    for (const p of paragraphs) {
        const trimmed = p.trim();
        if (!trimmed) {
            html += '<p><br></p>';
        } else {
            html += '<p>' + trimmed + '</p>';
        }
    }
    return html;
}

/**
 * 从章节 title 中提取纯标题（去掉 "第N章 " 前缀）
 */
function extractChapterTitle(title) {
    return (title || '').replace(/^第\s*[\d一二三四五六七八九十百千]+\s*章\s*/, '').trim();
}

// ======================== Action: get_book_list ========================

async function doGetBookList(cookieStr, input) {
    const { novelName } = input;
    log('info', 'get_book_list starting', { novelName });
    const browser = await launchBrowser(); let page;
    try {
        page = await browser.newPage(); await page.setViewport(CONFIG.VIEWPORT);
        await loginAndNavigate(page, cookieStr);

        const url = CONFIG.BASE_URL + '/api/pc/v1/book/book-list-v2';
        const result = await browserFetch(page, url, { method: 'GET' });
        log('info', 'book-list API response', { status: result.status, hasData: !!(result.json && result.json.data) });

        let books = [];
        if (result.json && result.json.data && Array.isArray(result.json.data.list)) {
            books = result.json.data.list;
        }
        log('info', 'books found', { count: books.length });

        return {
            success: true,
            action: 'get_book_list',
            books,
            total: result.json ? result.json.data.total : 0,
        };
    } finally {
        try {
            await Promise.race([
                browser.close(),
                new Promise((_, reject) => setTimeout(() => reject(new Error('close timeout')), 15_000))
            ]);
        } catch (e) {
            log('warn', 'browser close failed or timed out', { error: e.message });
            try { if (browser && browser.process()) browser.process().kill('SIGKILL'); } catch (_) {}
        }
        _globalBrowser = null;
        log('info', 'browser closed');
    }
}

// ======================== Action: get_book_option ========================

async function doGetBookOption(cookieStr, input) {
    log('info', 'get_book_option starting');
    const browser = await launchBrowser(); let page;
    try {
        page = await browser.newPage(); await page.setViewport(CONFIG.VIEWPORT);

        // 监听页面原生请求，也尝试直接用 page.goto 获取
        let responseBody = null;
        const onResponse = async (response) => {
            if (response.url().includes('/book/book-option') && !responseBody) {
                try { responseBody = await response.text(); } catch (_) {}
            }
        };
        page.on('response', onResponse);

        await loginAndNavigate(page, cookieStr);

        // 方式1: page.goto 直调 API，作为浏览器导航请求，headers 完整
        const t = Date.now();
        const apiUrl = CONFIG.BASE_URL + '/api/pc/v1/book/book-option?client_id=1&t=' + t;
        try {
            const gotoResp = await page.goto(apiUrl, { waitUntil: 'networkidle0', timeout: 15000 });
            if (!responseBody && gotoResp) {
                try { responseBody = await gotoResp.text(); } catch (_) {}
            }
        } catch (_) {}

        // 方式2: 如果方式1没拿到，等页面 JS 自己发起
        if (!responseBody) {
            for (let i = 0; i < 10; i++) {
                await new Promise(r => setTimeout(r, 500));
            }
        }
        page.off('response', onResponse);

        if (!responseBody) {
            log('warn', 'book-option response not intercepted, using fallback');
            return { success: true, action: 'get_book_option', category1: '301', category2: '333', pickedTagIds: '1,28,47' };
        }

        const data = JSON.parse(responseBody);
        const categoryList = (data && data.data && data.data.category_list) ? data.data.category_list : [];
        log('info', 'book-option intercepted', { textLen: responseBody.length, groupCount: categoryList.length });

        let cat1 = '301', cat2 = '333';
        for (const group of categoryList) {
            const cats = group.category || [];
            if (cats.length === 0) continue;
            const parent = cats[0];
            const children = parent.children || [];
            if (parent.id && children.length > 0) {
                cat1 = String(parent.id);
                cat2 = String(children[0].id);
                break;
            }
        }

        const tags = [];
        const seenTypes = new Set();
        for (const group of categoryList) {
            const tagInfos = group.tag_info || [];
            for (const ti of tagInfos) {
                if (seenTypes.has(ti.type_id)) continue;
                seenTypes.add(ti.type_id);
                const list = ti.select_list || [];
                if (list.length === 0) continue;
                let count = ti.can_choose_count;
                if (!count || count <= 0) count = 1;
                if (count > list.length) count = list.length;
                const shuffled = [...list].sort(() => Math.random() - 0.5);
                for (let i = 0; i < count; i++) {
                    tags.push(String(shuffled[i].tag_id || shuffled[i].id));
                }
            }
        }
        const tagIds = tags.length > 0 ? tags.join(',') : '1,28,47';

        log('info', 'book-option picked', { cat1, cat2, tagCount: tags.length });
        return {
            success: true,
            action: 'get_book_option',
            category1: cat1,
            category2: cat2,
            pickedTagIds: tagIds,
        };
    } finally {
        try {
            await Promise.race([
                browser.close(),
                new Promise((_, reject) => setTimeout(() => reject(new Error('close timeout')), 15_000))
            ]);
        } catch (e) {
            log('warn', 'browser close failed or timed out', { error: e.message });
            try { if (browser && browser.process()) browser.process().kill('SIGKILL'); } catch (_) {}
        }
        _globalBrowser = null;
        log('info', 'browser closed');
    }
}

// ======================== Action: get_platform_info ========================

async function doGetPlatformInfo(cookieStr, input) {
    const { novelName } = input;
    log('info', 'get_platform_info starting', { novelName });
    const browser = await launchBrowser(); let page;
    try {
        page = await browser.newPage(); await page.setViewport(CONFIG.VIEWPORT);
        await loginAndNavigate(page, cookieStr);

        // 1. 获取作品列表
        const listUrl = CONFIG.BASE_URL + '/api/pc/v1/book/book-list-v2';
        const listResult = await browserFetch(page, listUrl, { method: 'GET' });
        log('info', 'book-list API response', { status: listResult.status });

        let books = [];
        if (listResult.json && listResult.json.data && Array.isArray(listResult.json.data.list)) {
            books = listResult.json.data.list;
        }

        let foundBook = null;
        for (const b of books) {
            if (b.title === novelName) { foundBook = b; break; }
        }

        const result = {
            success: true,
            action: 'get_platform_info',
            bookExists: !!foundBook,
            bookId: foundBook ? foundBook.book_id : '',
            bookName: foundBook ? foundBook.title : '',
            books,
        };
        log('info', 'get_platform_info result', { bookExists: result.bookExists, bookId: result.bookId });
        return result;
    } finally {
        try {
            await Promise.race([
                browser.close(),
                new Promise((_, reject) => setTimeout(() => reject(new Error('close timeout')), 15_000))
            ]);
        } catch (e) {
            log('warn', 'browser close failed or timed out', { error: e.message });
            try { if (browser && browser.process()) browser.process().kill('SIGKILL'); } catch (_) {}
        }
        _globalBrowser = null;
        log('info', 'browser closed');
    }
}

// ======================== Action: create_book ========================

async function doCreateBook(cookieStr, input) {
    const {
        title, bookDesc, characters, category1, category2,
        tagIds, coverNum, clientId,
    } = input;
    log('info', 'create_book starting', { title });

    const browser = await launchBrowser(); let page;
    try {
        page = await browser.newPage(); await page.setViewport(CONFIG.VIEWPORT);
        await loginAndNavigate(page, cookieStr);

        const params = {
            book_activity_id: '0',
            book_desc: bookDesc || '精彩小说',
            book_id: '',
            book_status: '',
            book_updated_at: '',
            category_1: String(category1),
            category_2: String(category2),
            characters: characters || '',
            client_id: String(clientId || '3'),
            cover_num: coverNum || 5,
            cover_type: '2',
            is_girl: '0',
            is_over: '0',
            is_training: '0',
            modify_num: 3,
            pay_wishes_content: '',
            t: Date.now(),
            tag_ids: String(tagIds),
            title: title,
        };

        const url = CONFIG.BASE_URL + '/api/pc/v1/book/save-book-info';
        const body = buildFormBody(params);
        const result = await browserFetch(page, url, { method: 'POST', body });
        log('info', 'save-book-info API response', { status: result.status, hasData: !!(result.json && result.json.data) });

        const bookId = (result.json && result.json.data && result.json.data.book_id) ? result.json.data.book_id : '';
        if (!bookId) {
            log('warn', 'create book: no book_id in response', { response: result.json });
            return { success: false, action: 'create_book', error: 'no book_id in response' };
        }

        log('info', 'book created', { bookId, title });
        return { success: true, action: 'create_book', bookId };
    } finally {
        try {
            await Promise.race([
                browser.close(),
                new Promise((_, reject) => setTimeout(() => reject(new Error('close timeout')), 15_000))
            ]);
        } catch (e) {
            log('warn', 'browser close failed or timed out', { error: e.message });
            try { if (browser && browser.process()) browser.process().kill('SIGKILL'); } catch (_) {}
        }
        _globalBrowser = null;
        log('info', 'browser closed');
    }
}

// ======================== Action: set_book_info ========================

async function doSetBookInfo(cookieStr, input) {
    const {
        bookId, title, bookDesc, category1, category2, tagIds, characters, coverBase64,
    } = input;
    if (!bookId) { fail('missing required field: bookId'); return; }
    if (!coverBase64) { fail('missing required field: coverBase64'); return; }
    log('info', 'set_book_info starting', { bookId, title });

    const browser = await launchBrowser(); let page;
    try {
        page = await browser.newPage(); await page.setViewport(CONFIG.VIEWPORT);
        await loginAndNavigate(page, cookieStr);

        log('info', 'uploading cover image');
        const uploadResult = await page.evaluate(async (cb64) => {
            const bstr = atob(cb64);
            const bytes = new Uint8Array(bstr.length);
            for (let i = 0; i < bstr.length; i++) bytes[i] = bstr.charCodeAt(i);
            const pngBlob = new Blob([bytes], { type: 'image/png' });

            const jpegBlob = await new Promise((resolve, reject) => {
                const img = new Image();
                const url = URL.createObjectURL(pngBlob);
                img.onload = () => {
                    const canvas = document.createElement('canvas');
                    canvas.width = img.naturalWidth;
                    canvas.height = img.naturalHeight;
                    const ctx = canvas.getContext('2d');
                    ctx.drawImage(img, 0, 0);
                    canvas.toBlob((b) => {
                        URL.revokeObjectURL(url);
                        if (b) resolve(b);
                        else reject(new Error('canvas toBlob returned null'));
                    }, 'image/jpeg', 0.92);
                };
                img.onerror = () => { URL.revokeObjectURL(url); reject(new Error('png to jpeg: image load failed')); };
                img.src = url;
            });

            const fd = new FormData();
            fd.append('file', jpegBlob, 'cover.jpeg');
            fd.append('upload_type', 'book_cover');

            const resp = await fetch('https://zuozhe.qimao.com/api/pc/v1/upload', {
                method: 'POST', body: fd, credentials: 'include',
            });
            const data = await resp.json();
            const u = (data && data.data && data.data.files && data.data.files[0] && data.data.files[0].path) || '';
            if (!u) throw new Error('upload missing files[0].path in response: ' + JSON.stringify(data));

            const arrBuf = await jpegBlob.arrayBuffer();
            const arr = new Uint8Array(arrBuf);
            const chunks = [];
            const CHUNK = 8192;
            for (let i = 0; i < arr.length; i += CHUNK) {
                chunks.push(String.fromCharCode(...arr.subarray(i, i + CHUNK)));
            }
            const jpegBase64 = btoa(chunks.join(''));
            return { coverURL: u, jpegBase64: jpegBase64 };
        }, coverBase64);
        const coverURL = uploadResult.coverURL;
        log('info', 'cover uploaded', { path: coverURL });

        const jpegSize = Buffer.from(uploadResult.jpegBase64, 'base64').length;
        log('info', 'cover jpeg size from browser', { bookId: input.bookId, size: jpegSize });

        const params = {
            book_activity_id: '0',
            book_desc: bookDesc || '',
            book_id: bookId,
            book_status: '0',
            book_updated_at: Math.floor(Date.now() / 1000),
            category_1: String(category1 || ''),
            category_2: String(category2 || ''),
            characters: characters || '',
            client_id: '1',
            cover_ai_type: '0',
            cover_num: 5,
            cover_type: '3',
            is_girl: '0',
            is_over: '0',
            is_training: '0',
            modify_num: 3,
            pay_wishes_content: '',
            t: Date.now(),
            tag_ids: String(tagIds || ''),
            title: title || '',
            image_link: coverURL,
        };
        const url = CONFIG.BASE_URL + '/api/pc/v1/book/save-book-info';
        const body = buildFormBody(params);
        const result = await browserFetch(page, url, { method: 'POST', body });
        log('info', 'set_book_info save-book-info response', { status: result.status, text: result.text });

        if (!result.json || !result.json.data) {
            return { success: false, action: 'set_book_info', error: 'save-book-info returned no data' };
        }
        log('info', 'set_book_info success', { bookId });
        return { success: true, action: 'set_book_info', bookId };
    } finally {
        try {
            await Promise.race([
                browser.close(),
                new Promise((_, reject) => setTimeout(() => reject(new Error('close timeout')), 15_000))
            ]);
        } catch (e) {
            log('warn', 'browser close failed or timed out', { error: e.message });
            try { if (browser && browser.process()) browser.process().kill('SIGKILL'); } catch (_) {}
        }
        _globalBrowser = null;
        log('info', 'browser closed');
    }
}

// ======================== Action: save_draft ========================

async function doSaveDraft(cookieStr, input) {
    const { bookId, title, content, chapterNumber } = input;
    if (!bookId) { fail('missing required field: bookId'); return; }
    if (!content) { fail('missing required field: content'); return; }
    log('info', 'save_draft starting', { bookId, title, chapterNumber, contentLen: content.length });

    const chapterTitle = extractChapterTitle(title);
    const htmlContent = textToHtml(content);

    const browser = await launchBrowser(); let page;
    try {
        page = await browser.newPage(); await page.setViewport(CONFIG.VIEWPORT);
        await loginAndNavigate(page, cookieStr);

        const params = {
            aigc_apply_times: '0',
            aigc_list: [],
            author_say: '',
            book_id: bookId,
            chapter_content: htmlContent,
            chapter_id: '',
            chapter_index: '',
            chapter_name: chapterTitle,
            chapter_name_v2: '',
            chapter_type: '1',
            disclaimer_type: '1',
            is_vip: '',
            pc_version: '1.0.0',
            publish_type: '3',
            t: Date.now(),
            volume_content_type: '0',
            volume_id: '',
        };

        const url = CONFIG.BASE_URL + '/api/pc/v1/book-chapter/upload-chapter';
        log('info', 'save_draft params', { book_id: params.book_id, chapter_name: params.chapter_name, chapter_id: params.chapter_id, chapter_index: params.chapter_index, publish_type: params.publish_type, volume_id: params.volume_id, aigc_apply_times: params.aigc_apply_times, chapter_type: params.chapter_type });
        const body = JSON.stringify(params);
        const result = await browserFetch(page, url, { method: 'POST', body, headers: { 'Content-Type': 'application/json' } });
        log('info', 'upload-chapter API response', { status: result.status, body: result.json });

        if (result.status !== 200) {
            return { success: false, action: 'save_draft', error: 'upload-chapter HTTP ' + result.status };
        }
        if (!result.json || !result.json.data) {
            const apiCode = (result.json && result.json.code) ? result.json.code : 'unknown';
            const snippet = (result.text || '').substring(0, 500);
            log('warn', 'save_draft: no data in response', { apiCode, hasJson: !!result.json, responseSnippet: snippet });
            return { success: false, action: 'save_draft', error: 'upload-chapter no data, apiCode=' + apiCode };
        }

        log('info', 'save_draft success', { title: chapterTitle });
        return { success: true, action: 'save_draft' };
    } finally {
        try {
            await Promise.race([
                browser.close(),
                new Promise((_, reject) => setTimeout(() => reject(new Error('close timeout')), 15_000))
            ]);
        } catch (e) {
            log('warn', 'browser close failed or timed out', { error: e.message });
            try { if (browser && browser.process()) browser.process().kill('SIGKILL'); } catch (_) {}
        }
        _globalBrowser = null;
        log('info', 'browser closed');
    }
}

// ======================== Action: get_chapter_list ========================

async function doGetChapterList(cookieStr, input) {
    const { bookId } = input;
    if (!bookId) { fail('missing required field: bookId'); return; }
    log('info', 'get_chapter_list starting', { bookId });

    const browser = await launchBrowser(); let page;
    try {
        page = await browser.newPage(); await page.setViewport(CONFIG.VIEWPORT);
        await loginAndNavigate(page, cookieStr);

        const params = {
            book_id: bookId,
            order_type: '2',
            is_draft: '1',
            chapter_type: '1',
            t: Date.now(),
        };

        const url = CONFIG.BASE_URL + '/api/pc/v1/book-chapter/list';
        const queryString = buildFormBody(params);
        const result = await browserFetch(page, url + '?' + queryString, { method: 'GET' });
        log('info', 'chapter-list API response', { status: result.status });

        let chapters = [];
        let lastPublished = null;
        if (result.json && result.json.data) {
            chapters = result.json.data.list || [];
            lastPublished = result.json.data.last_book_text_chapter || null;
        }

        let maxNameIndex = 0;
        for (const ch of chapters) {
            if (ch.publish_type === '1' || ch.publish_type === 1) {
                const idx = parseInt(ch.name_index, 10) || 0;
                if (idx > maxNameIndex) maxNameIndex = idx;
            }
        }
        // 兜底: last_book_text_chapter
        if (lastPublished && lastPublished.name_index) {
            const idx = parseInt(lastPublished.name_index, 10) || 0;
            if (idx > maxNameIndex) maxNameIndex = idx;
        }

        log('info', 'chapter list parsed', {
            totalChapters: chapters.length,
            publishedCount: chapters.filter(c => c.publish_type === '1' || c.publish_type === 1).length,
            draftCount: chapters.filter(c => c.publish_type === '0' || c.publish_type === 0).length,
            maxNameIndex,
            hasLastPublished: !!lastPublished,
        });

        return {
            success: true,
            action: 'get_chapter_list',
            chapters,
            lastPublished,
            maxNameIndex,
        };
    } finally {
        try {
            await Promise.race([
                browser.close(),
                new Promise((_, reject) => setTimeout(() => reject(new Error('close timeout')), 15_000))
            ]);
        } catch (e) {
            log('warn', 'browser close failed or timed out', { error: e.message });
            try { if (browser && browser.process()) browser.process().kill('SIGKILL'); } catch (_) {}
        }
        _globalBrowser = null;
        log('info', 'browser closed');
    }
}

// ======================== Action: publish_draft ========================

async function doPublishDraft(cookieStr, input) {
    const { bookId, chapterId } = input;
    if (!bookId) { fail('missing required field: bookId'); return; }
    if (!chapterId) { fail('missing required field: chapterId'); return; }
    log('info', 'publish_draft starting', { bookId, chapterId });

    const browser = await launchBrowser(); let page;
    try {
        page = await browser.newPage(); await page.setViewport(CONFIG.VIEWPORT);
        await loginAndNavigate(page, cookieStr);

        const params = {
            book_id: bookId,
            chapter_id: chapterId,
            chapter_type: '1',
            is_update_chapter_name: 0,
            publish_type: '1',
            t: Date.now(),
        };

        const url = CONFIG.BASE_URL + '/api/pc/v1/book-chapter/draft-publish';
        const body = JSON.stringify(params);
        const result = await browserFetch(page, url, { method: 'POST', body, headers: { 'Content-Type': 'application/json' } });
        log('info', 'draft-publish API response', { status: result.status, hasData: !!(result.json && result.json.data) });

        if (result.status !== 200) {
            log('warn', 'draft-publish non-200', { status: result.status });
            return { success: false, action: 'publish_draft', error: 'draft-publish HTTP ' + result.status };
        }

        // 检查 data 字段
        if (!result.json || !result.json.data) {
            log('warn', 'draft-publish: data is empty', { response: result.text });
            return { success: false, action: 'publish_draft', error: 'draft-publish data empty or null' };
        }

        log('info', 'publish_draft success', { chapterId });
        const r = { success: true, action: 'publish_draft', chapterId };
        process.stdout.write(JSON.stringify(r) + '\n');
        return r;
    } finally {
        try {
            await Promise.race([
                browser.close(),
                new Promise((_, reject) => setTimeout(() => reject(new Error('close timeout')), 15_000))
            ]);
        } catch (e) {
            log('warn', 'browser close failed or timed out', { error: e.message });
            try { if (browser && browser.process()) browser.process().kill('SIGKILL'); } catch (_) {}
        }
        _globalBrowser = null;
        log('info', 'browser closed');
    }
}

// ======================== 主流程 ========================

async function main() {
    let input = await readStdinBase64();

    setLogPrefix(input.novelName || input.title || '');

    const action = input.action || 'get_platform_info';
    log('info', 'action selected', { action });

    const cookieStr = process.env.QIMAO_COOKIE;
    if (!cookieStr || cookieStr.trim() === '') {
        fail('QIMAO_COOKIE not set');
        return;
    }
    log('info', 'cookie loaded', { cookieLen: cookieStr.length });

    let lastError = null;
    for (let attempt = 0; attempt <= CONFIG.MAX_RETRIES; attempt++) {
        if (attempt > 0) {
            log('info', 'retry attempt', { attempt });
            await sleep(2000);
        }
        try {
            let result;
            switch (action) {
                case 'get_book_list':
                    result = await doGetBookList(cookieStr, input);
                    break;
                case 'get_book_option':
                    result = await doGetBookOption(cookieStr, input);
                    break;
                case 'get_platform_info':
                    result = await doGetPlatformInfo(cookieStr, input);
                    break;
                case 'create_book':
                    result = await doCreateBook(cookieStr, input);
                    break;
                case 'set_book_info':
                    result = await doSetBookInfo(cookieStr, input);
                    break;
                case 'save_draft':
                    result = await doSaveDraft(cookieStr, input);
                    break;
                case 'get_chapter_list':
                    result = await doGetChapterList(cookieStr, input);
                    break;
                case 'publish_draft':
                    result = await doPublishDraft(cookieStr, input);
                    log('info', 'publish success', result);
                    return;
                default:
                    fail('unknown action: ' + action);
                    return;
            }
            log('info', 'publish success', result);
            output(result);
            return;
        } catch (e) {
            lastError = e;
            log('warn', 'attempt failed', { attempt, error: e.message });
        }
    }

    fail(lastError ? lastError.message : 'unknown error after retries');
}

main().catch(e => {
    log('error', 'unhandled exception', { error: e.message, stack: e.stack });
    process.exit(1);
});
