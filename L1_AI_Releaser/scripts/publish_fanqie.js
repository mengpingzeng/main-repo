/**
 * publish_fanqie.js — 番茄小说章节发布脚本 (Puppeteer 浏览器自动化)
 *
 * 运行环境：Linux 服务器，无显示器，无图形界面
 * 启动参数：headless + --no-sandbox --disable-setuid-sandbox --disable-dev-shm-usage
 *
 * 入参（两种模式）：
 *   直接模式：node publish_fanqie.js '<JSON字符串>'
 *   base64 模式：node publish_fanqie.js --base64 （从 stdin 读取 base64 编码的 JSON）
 *
 * Cookie 从环境变量 FANQIE_COOKIE 读取
 *
 * 输出（stdout）：JSON 一行
 *   {"success":true,"postId":"chapter_id_xxx"}
 *   {"success":false,"error":"error description"}
 *
 * 依赖：npm install puppeteer
 */

'use strict';

const puppeteer = require('puppeteer');

// ======================== 日志 ========================

function log(level, msg, data) {
    const entry = { time: new Date().toISOString(), level, msg, ...(data ? { data } : {}) };
    process.stderr.write(JSON.stringify(entry) + '\n');
}

// ======================== 配置 ========================

const CONFIG = {
    BASE_URL: 'https://fanqienovel.com',
    WRITER_URL: 'https://fanqienovel.com/main/writer/',
    TIMEOUT_MS: 300000,
    NAVIGATION_TIMEOUT: 60000,
    ELEMENT_TIMEOUT: 15000,
    MAX_RETRIES: 1,
    VIEWPORT: { width: 1920, height: 1080 },
    MIN_CONTENT_LENGTH: 1000, // 正文最低字数
};

// ======================== 工具函数 ========================

function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

/**
 * 从 stdin 读取 base64 编码的 JSON 入参，解码并解析。
 * 配合 mcp_server.js 使用，规避命令行传递长文本/特殊字符的问题。
 * @returns {Promise<object>} 解析后的入参对象
 */
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

// ======================== 主流程 ========================

async function main() {
    // 1. 解析入参（支持两种模式）
    const isBase64 = process.argv.includes('--base64');
    let input;

    if (isBase64) {
        // --- base64 模式：从 stdin 读取 base64 编码的 JSON ---
        // 配合 mcp_server.js 使用，规避命令行参数中的换行符/引号截断问题
        input = await readStdinBase64();
    } else {
        // --- 直接模式：从命令行参数读取原始 JSON ---
        // 用于手动测试：node publish_fanqie.js '{"title":"xx",...}'
        try {
            const raw = process.argv[2];
            if (!raw) {
                fail('missing argument: JSON input or --base64 flag required');
                return;
            }
            input = JSON.parse(raw);
        } catch (e) {
            fail('invalid JSON input: ' + e.message);
            return;
        }
    }

    const { title, content, novelName, volumeName, chapterNumber } = input;
    if (!title)      { fail('missing required field: title'); return; }
    if (!content)    { fail('missing required field: content'); return; }
    if (!novelName)  { fail('missing required field: novelName'); return; }

    log('info', 'input parsed', { title, novelName, contentLen: content.length });

    // 2. 读取 Cookie
    const cookieStr = process.env.FANQIE_COOKIE;
    if (!cookieStr || cookieStr.trim() === '') {
        fail('FANQIE_COOKIE not set');
        return;
    }
    log('info', 'cookie loaded', { cookieLen: cookieStr.length });

    // 3. 执行发布（支持重试）
    let lastError = null;
    for (let attempt = 0; attempt <= CONFIG.MAX_RETRIES; attempt++) {
        if (attempt > 0) {
            log('info', 'retry attempt', { attempt });
            await sleep(2000);
        }
        try {
            const result = await publishWithTimeout(cookieStr, input);
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

let _globalBrowser = null; // 用于超时强制清理

function publishWithTimeout(cookieStr, input) {
    return new Promise((resolve, reject) => {
        const timer = setTimeout(() => {
            // 超时时强制关闭浏览器，防止孤儿 Chrome 进程残留
            if (_globalBrowser) {
                _globalBrowser.close().catch(() => {});
                _globalBrowser = null;
            }
            reject(new Error('overall timeout exceeded ' + CONFIG.TIMEOUT_MS + 'ms'));
        }, CONFIG.TIMEOUT_MS);

        doPublish(cookieStr, input)
            .then(result => { clearTimeout(timer); resolve(result); })
            .catch(err  => { clearTimeout(timer); reject(err); });
    });
}

// ======================== 核心发布逻辑 ========================

async function doPublish(cookieStr, input) {
    const { title, content, novelName } = input;

    // 字数校验
    if (content.length < CONFIG.MIN_CONTENT_LENGTH) {
        throw new Error('内容不足' + CONFIG.MIN_CONTENT_LENGTH + '字（当前 ' + content.length + ' 字）');
    }

    log('info', 'launching browser');
    const browser = await puppeteer.launch({
        headless: 'new',
        protocolTimeout: 180000,
        args: [
            '--no-sandbox',
            '--disable-setuid-sandbox',
            '--disable-dev-shm-usage',
            '--disable-gpu',
            '--disable-extensions',
            '--disable-background-networking',
            '--disable-sync',
            '--no-first-run',
            '--disable-features=TranslateUI',
            '--disable-software-rasterizer',
            '--memory-pressure-off',
            '--js-flags=--max-old-space-size=256',
        ],
    });

    _globalBrowser = browser;
    let page;
    try {
        page = await browser.newPage();
        await page.setViewport(CONFIG.VIEWPORT);

        // 自动接受所有原生浏览器弹窗 (alert/confirm/prompt)，防止阻塞发布流程
        page.on('dialog', async (dialog) => {
            log('info', 'auto-accepting native dialog', { type: dialog.type(), message: dialog.message() });
            await dialog.accept();
        });

        // === 1. 注入 Cookie 并打开作家专区 ===
        log('info', 'setting cookies and navigating to writer');
        const cookies = parseCookieString(cookieStr, CONFIG.BASE_URL);
        await page.setCookie(...cookies);
        await page.goto(CONFIG.WRITER_URL, { waitUntil: 'domcontentloaded', timeout: CONFIG.NAVIGATION_TIMEOUT });
        await sleep(5000);

        // 确认已登录
        const loggedIn = await checkLogin(page);
        if (!loggedIn) {
            throw new Error('login failed: cookie may be expired, page redirected to login');
        }
        log('info', 'login confirmed');

        // === 2. 点击"作品管理" ===
        await clickButton(page, ['作品管理', '我的作品', '作品', '管理作品']);
        await sleep(2000);

        // === 3. 查找或创建作品 ===
        const workIdBefore = await page.evaluate(() => {
            const m = window.location.href.match(/writer\/(\d+)/);
            return m ? m[1] : null;
        });

        let foundNovel = await findNovelByName(page, novelName);
        if (foundNovel) {
            log('info', 'novel found, entering chapter management', { novelName });
            await clickChapterManagement(page, novelName);
        } else {
            log('info', 'novel not found, creating new via direct URL', { novelName });

            // Navigate directly to the novel creation page
            await page.goto('https://fanqienovel.com/main/writer/create', { waitUntil: 'domcontentloaded', timeout: CONFIG.NAVIGATION_TIMEOUT });
            await sleep(5000);

            // Set a generous default timeout for slow page interactions
            page.setDefaultTimeout(60000);

            // Wait for the creation form to actually render
            try {
                await page.waitForFunction(() => {
                    const el = document.querySelector('input[placeholder*="作品名称"]');
                    return el && el.offsetParent !== null;
                }, { timeout: 60000 });
            } catch (e) {
                log('warn', 'create form not ready after 60s, continuing anyway');
            }

            // Dismiss any welcome modals
            await dismissWelcomeModal(page);

            // Fill the creation form (returns workId on success, null on failure/name conflict)
            let createdWorkId = await createNovelViaForm(page, novelName);

            // Name conflict retry with alt names
            const altNames = [novelName + '之续', novelName + '新篇'];
            let altIdx = 0;
            while (!createdWorkId && altIdx < altNames.length) {
                const altName = altNames[altIdx];
                altIdx++;
                log('info', 'retrying novel creation with alt name', { altName });
                // Navigate back to create page and retry
                await page.goto('https://fanqienovel.com/main/writer/create', { waitUntil: 'domcontentloaded', timeout: CONFIG.NAVIGATION_TIMEOUT });
                await sleep(3000);
                await dismissWelcomeModal(page);
                await createNovelViaForm(page, altName);
                // Check URL for workId
                createdWorkId = await page.evaluate(() => {
                    const m = window.location.href.match(/writer\/(\d+)/);
                    return m ? m[1] : null;
                });
                if (createdWorkId) {
                    log('info', 'novel created with alt name', { altName, workId: createdWorkId });
                    break;
                }
            }

            if (createdWorkId && createdWorkId !== workIdBefore) {
                log('info', 'novel created successfully', { novelName, workId: createdWorkId });
            } else {
                // Fallback: go back to writer page and search
                log('warn', 'novel creation verification failed, searching for novel');
                await page.goto('https://fanqienovel.com/main/writer/', { waitUntil: 'domcontentloaded', timeout: CONFIG.NAVIGATION_TIMEOUT });
                await sleep(3000);
                await dismissWelcomeModal(page);
                await clickButton(page, ['作品管理']);
                await sleep(2000);
                foundNovel = await findNovelByName(page, novelName);
                if (foundNovel) {
                    log('info', 'novel found after creation, using existing', { novelName });
                    await clickChapterManagement(page, novelName);
                } else {
                    throw new Error('无法创建或找到作品: ' + novelName);
                }
            }
        }
        await sleep(2000);

        // === 3.5 关闭可能出现的弹窗（读者纠错功能引导等） ===
        await dismissPopups(page);
        await sleep(1000);

        // === 3.6 确定章节序号：入参有效则直接用，否则自动尾章 +1 ===
        let effectiveChapterNumber;
        if (!input.chapterNumber || input.chapterNumber === 0) {
            const lastNum = await getLastChapterNumber(page);
            effectiveChapterNumber = lastNum + 1;
            log('info', 'auto-incremented chapter number', { lastNum, newNum: effectiveChapterNumber });
        } else {
            effectiveChapterNumber = input.chapterNumber;
            log('info', 'using provided chapter number', { chapterNumber: input.chapterNumber });
        }

        // === 4. 导航到章节编辑器（新建章节的 <a> 有 target=_blank，需直接导航） ===
        await navigateToChapterEditor(page);
        await sleep(2000);

        // === 4.5 进入编辑器后关闭弹窗，并确认已进入编辑页 ===
        await dismissPopups(page);
        await sleep(1000);

        // 检查是否进入编辑器（有标题输入框），若没有则重试
        const onEditor = await isOnEditorPage(page);
        if (!onEditor) {
            log('warn', 'not on editor page, retrying navigate to editor');
            await navigateToChapterEditor(page);
            await sleep(2000);
            await dismissPopups(page);
            await sleep(1000);
        }

        // === 5. 填写章节内容 ===
        log('info', 'filling chapter number', { chapterNumber: effectiveChapterNumber });
        await fillChapterNumber(page, effectiveChapterNumber);

        log('info', 'filling chapter title', { title });
        await fillTitle(page, title);

        log('info', 'filling chapter content', { contentLen: content.length });
        await fillContent(page, content);
        await sleep(1000);

        // === 6. 调试：保存页面截图（仅 viewport，不做全页截取避免卡顿） ===
        try {
            await page.screenshot({ path: '/tmp/fanqie_debug.png', fullPage: false });
        } catch (e) {
            log('warn', 'debug save failed', { error: e.message });
        }

        // === 6. 点击"下一步" ===
        log('info', 'clicking next step button');
        await clickButton(page, ['下一步']);
        await sleep(2000);

        // === 6.5 点击"下一步"后可能直接弹出"发布提示"确认弹窗 ===
        // 流程：点下一步 → 发布提示 → 点确认发布 → 继续
        const afterNextModal = await handlePublishConfirmModal(page);
        if (afterNextModal) {
            log('info', 'publish confirm modal handled right after next step');
            await sleep(2000);
        }

        // === 6.6 处理"请选择内容检测方式"弹窗（新版番茄新增） ===
        // 点击"下一步"后可能出现内容检测方式选择弹窗：
        //   "请选择内容检测方式"（全面检测 / 基础检测）
        // 点击"仅基础检测"快速跳过
        await handleContentDetectionDialog(page);
        await sleep(1000);

        // === 7. 处理错别字检查弹窗 ===
        // 点击"下一步"后页面进入错别字检测，"忽略全部"后（或检测到0错误时），
        // 需要再次点击"下一步"才能进入发布设置页面。
        const typoHandled = await handleTypoCheckAndNext(page);
        if (!typoHandled) {
            // 检测无错误或无"忽略全部"按钮，仍需点击"下一步"进入设置页
            log('info', 'no typo dialog or zero errors, clicking next step to proceed');
            await sleep(1000);
            await clickButton(page, ['下一步']);
            await sleep(2000);
        }

        // === 8. 发布设置页面：选分卷 → AI选否 → 确认定时关闭 ===
        log('info', 'handling publish settings page');
        await handlePublishSettings(page);
        await sleep(2000);

        // === 9. 简化发布提交流程：不断点击当前页面可见的「下一步」或「确认发布」按钮 ===
        // DEBUG: screenshot and dump buttons before submit loop
        try {
            const preSubmit = await page.evaluate(() => ({
                url: window.location.href,
                buttons: Array.from(document.querySelectorAll('button')).filter(b => b.offsetParent !== null && !b.disabled).map(b => b.textContent.trim()).filter(t => t.length > 0 && t.length < 25),
                inputs: Array.from(document.querySelectorAll('input[type="text"], textarea, input[type="number"]')).filter(el => el.offsetParent !== null).map(el => ({ placeholder: el.placeholder || '', value: el.value ? el.value.substring(0,20) : '' }))
            }));
        } catch (e) {}

        log('info', 'entering simplified publish submit loop');
        let publishSuccessConfirmed = false;
        const maxSubmitAttempts = 12;
        for (let submitAttempt = 0; submitAttempt < maxSubmitAttempts; submitAttempt++) {
            // Check for success first
            try {
                const successCheck = await page.evaluate(() => {
                    const txt = document.body.innerText;
                    if (txt.includes('发布成功') || txt.includes('成功发布') || txt.includes('已发布')) return 'text';
                    const url = window.location.href;
                    if (url.includes('/chapter/') || url.includes('chapter_id=')) return 'url_chapter';
                    return null;
                });
                if (successCheck) {
                    log('info', 'publish success confirmed', { indicator: successCheck });
                    publishSuccessConfirmed = true;
                    break;
                }
            } catch (e) {}

            // Handle confirm modal if present
            try {
                const modalHandled = await page.evaluate(() => {
                    const dialogTexts = ['发布提示', '是否确定提交', '确认发布', '确定发布'];
                    const allSpans = document.querySelectorAll('span, div, p');
                    for (const el of allSpans) {
                        const t = el.textContent.trim();
                        for (const dt of dialogTexts) {
                            if (t.includes(dt) && el.offsetParent !== null) {
                                const container = el.closest('div[class*="modal"], div[class*="dialog"], div[class*="drawer"]') || document.body;
                                const btns = container.querySelectorAll('button');
                                for (const btn of btns) {
                                    const bt = btn.textContent.trim();
                                    if (['确认发布', '确认', '确定', '提交', '确定提交'].includes(bt) && btn.offsetParent !== null) {
                                        btn.click();
                                        return 'clicked:' + bt;
                                    }
                                }
                            }
                        }
                    }
                    return null;
                });
                if (modalHandled) {
                    log('info', 'publish confirm modal handled', { action: modalHandled });
                    await sleep(3000);
                    continue;
                }
            } catch (e) {}

            // Click the best available button
            let clicked = await page.evaluate(() => {
                const btns = document.querySelectorAll('button');
                for (const btn of btns) {
                    if (btn.offsetParent === null || btn.disabled) continue;
                    const t = btn.textContent.trim();
                    if (t === '提交' || t === '确认发布' || t === '发布') { btn.click(); return t; }
                }
                for (const btn of btns) {
                    if (btn.offsetParent === null || btn.disabled) continue;
                    const t = btn.textContent.trim();
                    if (t === '下一步') { btn.click(); return '下一步'; }
                }
                return null;
            });

            if (!clicked) {
                try {
                    const fnd = await page.evaluate(() => {
                        const xpath = "//button[text()='提交']";
                        const el = document.evaluate(xpath, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
                        if (el) { el.click(); return 'xpath_提交'; }
                        return null;
                    });
                    if (fnd) clicked = fnd;
                } catch (e) {}
            }

            if (clicked) {
                log('info', 'publish step clicked', { action: clicked, attempt: submitAttempt });
            } else {
                log('warn', 'no publish button found at step', { attempt: submitAttempt });
            }
            await sleep(3000);
        }

        if (!publishSuccessConfirmed) {
            try { await page.screenshot({ path: '/tmp/fanqie_publish_fail.png', fullPage: false }); } catch (se) {}
            throw new Error('发布失败：未检测到发布成功标识（"发布成功"文案或页面跳转），当前页面可能卡在弹窗或设置步骤，请检查截图 /tmp/fanqie_publish_fail.png');
        }

        let postId = await getChapterIdFromURL(page);

        if (!postId) {
            try { await page.screenshot({ path: '/tmp/fanqie_publish_fail.png', fullPage: false }); } catch (se) {}
            throw new Error('发布失败：虽然检测到发布成功标识，但无法从页面提取章节 ID，当前页面 URL: ' + page.url());
        }

        log('info', 'chapter published', { postId });
        return { success: true, postId };

    } finally {
        await browser.close().catch(() => {});
        _globalBrowser = null;
        log('info', 'browser closed');
    }
}

// ======================== Cookie ========================

function parseCookieString(cookieStr, domain) {
    const domainClean = domain.replace(/^https?:\/\//, '').split('/')[0];
    const attrNames = new Set(['expires', 'path', 'domain', 'secure', 'httponly',
        'samesite', 'max-age', 'priority', 'partitioned', 'size', 'sameparty']);
    return cookieStr.split(';').map(pair => {
        const eqIdx = pair.indexOf('=');
        const name = eqIdx >= 0 ? pair.substring(0, eqIdx).trim() : pair.trim();
        const value = eqIdx >= 0 ? pair.substring(eqIdx + 1).trim() : '';
        if (attrNames.has(name.toLowerCase())) return null;
        return { name, value, domain: '.' + domainClean, path: '/' };
    }).filter(c => c && c.name && c.value);
}

// ======================== 页面操作 ========================

async function checkLogin(page) {
    try {
        // 等待"工作台"文字出现，说明已登录
        await page.waitForFunction(() => {
            return document.body && document.body.innerText && document.body.innerText.includes('工作台');
        }, { timeout: 15000 });
        log('info', '工作台 loaded');
        return true;
    } catch (e) {
        log('warn', 'checkLogin failed', { url: page.url(), error: e.message });
        try {
            await page.screenshot({ path: '/tmp/fanqie_login_fail.png', fullPage: false });
            const body = await page.evaluate(() => document.body ? document.body.innerText.substring(0, 500) : 'NO_BODY');
            log('warn', 'login check body', { body });
        } catch (se) {}
        return false;
    }
}

/**
 * 点击包含指定文字的按钮（精确匹配按钮文字）。
 * text 可以是字符串或字符串数组（依次尝试）。
 */
async function clickButton(page, text) {
    const texts = Array.isArray(text) ? text : [text];

    for (let i = 0; i < texts.length; i++) {
        const targetText = texts[i];
        const isLast = (i === texts.length - 1);

        const clicked = await page.evaluate((targetText) => {
            const candidates = document.querySelectorAll('button, a, span, div[class*="btn"], div[class*="button"]');
            for (const el of candidates) {
                const elText = (el.textContent || '').trim();
                if (elText === targetText) {
                    el.click();
                    return true;
                }
            }
            for (const el of candidates) {
                const elText = (el.textContent || '').trim();
                if (elText.includes(targetText) && elText.length <= targetText.length + 4) {
                    el.click();
                    return true;
                }
            }
            return false;
        }, targetText);

        if (clicked) {
            log('info', 'button clicked', { text: targetText });
            return;
        }

        if (!isLast) {
            log('warn', 'button not found, trying next', { tried: targetText, next: texts[i + 1] });
        }
    }

    // 所有候选文本都失败，用最后一个文本尝试 xpath
    const lastText = texts[texts.length - 1];
    const xpath = `//button[contains(normalize-space(),"${lastText}")]|//span[contains(normalize-space(),"${lastText}")]|//a[contains(normalize-space(),"${lastText}")]`;
    const clickedViaXpath = await page.evaluate((xpathStr) => {
        const result = document.evaluate(
            xpathStr, document, null,
            XPathResult.FIRST_ORDERED_NODE_TYPE, null
        );
        const el = result.singleNodeValue;
        if (el) { el.click(); return true; }
        return false;
    }, xpath);
    if (clickedViaXpath) {
        log('info', 'button clicked via xpath', { text: lastText });
        return;
    }

    // 最终兜底：打印页面按钮列表以便调试
    const btnList = await page.evaluate(() => {
        const candidates = document.querySelectorAll('button, a, span, div[class*="btn"], div[class*="button"], [role="button"]');
        return Array.from(candidates).slice(0, 40).map(el => (el.textContent || '').trim().replace(/\s+/g, ' ')).filter(t => t.length > 0 && t.length < 30);
    });
    log('error', 'all button texts on page', { btnList });
    throw new Error('找不到按钮: ' + JSON.stringify(texts));
}

/**
 * 在作品列表中查找同名作品。
 */
async function findNovelByName(page, novelName) {
    try {
        const found = await page.evaluate((name) => {
            const name15 = name.substring(0, 15);
            const links = document.querySelectorAll('a, span, div, p, h1, h2, h3, h4, h5, h6');
            for (const el of links) {
                const text = (el.textContent || '').trim();
                if (text === name || text === name15 || text.includes(name)) {
                    return true;
                }
            }
            return false;
        }, novelName);
        return found;
    } catch {
        return false;
    }
}

/**
 * 点击指定作品的"章节管理"按钮。
 */
async function clickChapterManagement(page, novelName) {
    try {
        const clicked = await page.evaluate((name) => {
            // 找到包含作品名的行，再找其中"章节管理"按钮
            const rows = document.querySelectorAll('tr, li, div[class*="row"], div[class*="item"], div[class*="card"]');
            for (const row of rows) {
                if (row.textContent && row.textContent.includes(name)) {
                    const btns = row.querySelectorAll('button, a, span');
                    for (const btn of btns) {
                        if (btn.textContent && btn.textContent.trim() === '章节管理') {
                            btn.click();
                            return true;
                        }
                    }
                }
            }
            return false;
        }, novelName);

        if (!clicked) {
            // 回退：在整页中找"章节管理"
            await clickButton(page, ['章节管理', '管理章节', '章节列表', '章节目录']);
        }
    } catch (e) {
        log('warn', 'clickChapterManagement fallback', { error: e.message });
        await clickButton(page, ['章节管理', '管理章节', '章节列表', '章节目录']);
    }
}

/**
 * 关闭平台升级引导弹窗（如"番茄原创平台全新上线"提示）。
 */
async function dismissWelcomeModal(page) {
    try {
        const btns = await page.$$('button');
        for (const btn of btns) {
            try {
                const t = await btn.evaluate(el => (el.textContent || '').trim());
                if (t === '立即体验' && (await btn.evaluate(el => el.offsetParent !== null))) {
                    await btn.click();
                    log('info', 'welcome modal dismissed');
                    await sleep(2000);
                    return true;
                }
            } catch (e) {}
        }
    } catch (e) {}
    return false;
}

/**
 * 通过直接导航到 /main/writer/create 页面创建新书。
 */
async function createNovelViaForm(page, novelName) {
    await sleep(2000);

    // 1. Fill novel name (max 15 chars)
    const nameInput = await page.$('input[placeholder*="作品名称"]');
    if (!nameInput) throw new Error('找不到书名输入框');
    await nameInput.click();
    await nameInput.evaluate(el => el.value = '');
    await nameInput.type(novelName.substring(0, 15), { delay: 30 });
    log('info', 'novel name filled', { novelName });
    await sleep(500);

    // 2. Select channel: 男频 (value=1)
    try {
        await page.evaluate(() => {
            const r = document.querySelector('input[name="pindao"][value="1"]');
            if (r) { r.checked = true; r.dispatchEvent(new Event('change', { bubbles: true })); }
        });
        log('info', 'channel 男频 selected');
    } catch (e) {}
    await sleep(500);

    // 3. Select work tags (作品标签)
    try {
        const tagResult = await page.evaluate(() => {
            const allSpans = document.querySelectorAll('span, div');
            for (const el of allSpans) {
                if (el.textContent.trim() === '请选择作品标签' && el.offsetParent !== null) {
                    el.click();
                    return { clicked: true, tag: el.tagName };
                }
            }
            // Also try clicking the select component
            const tagInput = document.querySelector('.arco-select-view input, input[placeholder*="标签"]');
            if (tagInput && tagInput.offsetParent !== null) {
                tagInput.click();
                return { clicked: true, tag: 'input' };
            }
            return { clicked: false };
        });
        if (tagResult.clicked) {
            await sleep(1500);
            // Click first available tag option from dropdown
            const tagClicked = await page.evaluate(() => {
                const options = document.querySelectorAll('[class*="select-option"], [class*="option"], li[class*="select"], div[class*="dropdown"] span, div[class*="dropdown"] div');
                for (const opt of options) {
                    if (opt.offsetParent !== null && opt.textContent.trim().length > 1 && opt.textContent.trim().length < 10) {
                        opt.click();
                        return opt.textContent.trim();
                    }
                }
                return null;
            });
            log('info', 'work tag selected', { tag: tagClicked || '(none)' });
        }
    } catch (e) {
        log('warn', 'tag selection skipped', { error: e.message });
    }
    await sleep(500);

    // 4. Fill protagonist names (max 5 chars each)
    try {
        const p1 = await page.$('input[placeholder*="主角名1"]');
        if (p1) { await p1.evaluate(el => el.value = ''); await p1.type('周明远'.substring(0, 5), { delay: 20 }); }
    } catch (e) {}
    try {
        const p2 = await page.$('input[placeholder*="主角名2"]');
        if (p2) { await p2.evaluate(el => el.value = ''); await p2.type('林秀兰'.substring(0, 5), { delay: 20 }); }
    } catch (e) {}
    await sleep(300);

    // 5. Fill synopsis (50-500 chars required)
    const synopsisText = '一个在工地上干了十二年的农民工周明远，在老板跑路后身无分文。面对欠租的压力和女儿期盼的目光，他偶然拾起一张编辑名片，用那双搬过砖绑过钢筋的粗糙双手，在深夜的出租屋里敲下人生的第一个字。这是一部关于底层小人物不甘沉沦、用文字对抗命运的奋斗史。当城市的天黑透时，文字就是唯一的光。';
    try {
        const synopsis = await page.$('textarea[placeholder*="作品简介"]');
        if (synopsis) {
            await synopsis.evaluate(el => el.value = '');
            await synopsis.type(synopsisText, { delay: 10 });
        }
    } catch (e) {}
    await sleep(500);

    // 6. Click "立即创建" to submit
    const submitted = await page.evaluate(() => {
        const btns = document.querySelectorAll('button');
        for (const btn of btns) {
            if (btn.offsetParent === null) continue;
            if ((btn.textContent || '').trim() === '立即创建') {
                btn.click();
                return true;
            }
        }
        return false;
    });
    if (!submitted) throw new Error('找不到立即创建按钮');
    log('info', 'novel creation submitted');

    // 7. Wait for response and verify
    await sleep(3000);
    await dismissWelcomeModal(page);
    await sleep(2000);
    // Check for name conflict error
    const nameConflict = await page.evaluate(() => {
        return document.body.innerText.includes('该书名已存在') ||
               document.body.innerText.includes('书名已存在') ||
               document.body.innerText.includes('已存在');
    });
    if (nameConflict) {
        log('warn', 'novel name conflict detected, will retry with alt name');
        // Return null workId to signal retry
        return null;
    }
    // Check URL for workId — if navigation happened
    let workId = await page.evaluate(() => {
        const m = window.location.href.match(/writer\/(\d+)/);
        return m ? m[1] : null;
    });
    if (workId) {
        log('info', 'novel created, workId in URL', { workId });
        return workId;
    } else {
        // AJAX creation — wait longer and check for success notifications
        log('info', 'checking for AJAX creation result');
        await sleep(3000);
        const success = await page.evaluate(() => {
            const el = document.querySelector('[class*="success"], [class*="toast"], .arco-notification');
            if (el && el.offsetParent !== null) return el.textContent.trim().substring(0, 60);
            return null;
        });
        log('info', 'creation result message', { message: success || '(none)' });
        return null; // Can't confirm, signal to fallback search
    }
}

/**
 * 处理创建新书时的中间步骤（保留兼容旧流程）。
 */
async function handleIntermediateCreateStep(page) {}

/**
 * 填写章节序号（位于标题上方，"第 X 章"格式）。
 * React 控件需要触发原生 input/change 事件才能生效。
 */
async function fillChapterNumber(page, number) {
    const numStr = String(number);

    try {
        const filled = await page.evaluate((num) => {
            const nativeInputValueSetter = Object.getOwnPropertyDescriptor(
                window.HTMLInputElement.prototype, 'value'
            ).set;

            const inputs = document.querySelectorAll('input[type="text"]');
            for (const inp of inputs) {
                if (inp.value !== '' && inp.value.length > 1) continue;
                let parent = inp.parentElement;
                for (let i = 0; i < 5 && parent; i++) {
                    const text = parent.textContent || '';
                    if (text.includes('第') && text.includes('章')) {
                        nativeInputValueSetter.call(inp, num);
                        inp.dispatchEvent(new Event('input', { bubbles: true }));
                        inp.dispatchEvent(new Event('change', { bubbles: true }));
                        inp.dispatchEvent(new Event('blur', { bubbles: true }));
                        return true;
                    }
                    parent = parent.parentElement;
                }
            }
            return false;
        }, numStr);

        if (filled) {
            log('info', 'chapter number filled');
            return;
        }
    } catch (e) {
        log('warn', 'chapter number fill error', { error: e.message });
    }

    log('warn', 'chapter number input not found');
}

/**
 * 在章节管理/目录页面读取最后一章的序号。
 * 要求在调用前已进入作品章节目录页面（点击"章节管理"后）。
 * @param {Page} page
 * @returns {Promise<number>} 最后一章序号，若无章节则返回 0
 */
async function getLastChapterNumber(page) {
    try {
        await sleep(2000);

        const lastNum = await page.evaluate(() => {
            const bodyText = document.body.innerText || '';

            const matches = bodyText.match(/第(\d+)章/g);
            if (matches && matches.length > 0) {
                let maxNum = 0;
                for (const m of matches) {
                    const num = parseInt(m.match(/(\d+)/)[1], 10);
                    if (!isNaN(num) && num > maxNum) {
                        maxNum = num;
                    }
                }
                if (maxNum > 0) return maxNum;
            }

            const candidates = document.querySelectorAll('tr, li, div[class*="chapter"], div[class*="row"], div[class*="item"]');
            const nums = [];
            for (const el of candidates) {
                const text = (el.textContent || '').trim();
                const m = text.match(/^(\d+)[\s\.、]/);
                if (m) {
                    nums.push(parseInt(m[1], 10));
                }
            }
            if (nums.length > 0) {
                return Math.max(...nums);
            }

            const links = document.querySelectorAll('a');
            const linkNums = [];
            for (const a of links) {
                const text = (a.textContent || '').trim();
                const m = text.match(/^(\d+)/);
                if (m) {
                    linkNums.push(parseInt(m[1], 10));
                }
            }
            if (linkNums.length > 0) {
                return Math.max(...linkNums);
            }

            return 0;
        });

        log('info', 'last chapter number', { lastNum });
        return typeof lastNum === 'number' ? lastNum : 0;
    } catch (e) {
        log('warn', 'getLastChapterNumber error', { error: e.message });
        return 0;
    }
}

/**
 * 填写章节标题。
 */
async function fillTitle(page, title) {
    const selectors = [
        'input[placeholder*="书名"]',
        'input[placeholder*="章节名"]',
        'input[placeholder*="标题"]',
        'input[name*="title"]',
        'input[name*="chapterName"]',
        'input[class*="title"]',
    ];

    for (const sel of selectors) {
        const el = await page.$(sel).catch(() => null);
        if (el) {
            await el.click();
            await el.evaluate(e => e.value = '');
            await el.type(title, { delay: 20 });
            log('info', 'title filled', { selector: sel });
            return;
        }
    }

    // Label-based positioning: find input near label containing "标题" or "章节"
    const labelInput = await page.evaluateHandle(() => {
        const labels = document.querySelectorAll('label, span, div[class*="label"], div[class*="form-item"]');
        for (const label of labels) {
            const text = label.textContent.trim();
            if (text.includes('标题') || text.includes('章节名') || text.includes('名称')) {
                const inputEl = label.closest('div, form, .form-item')?.querySelector('input[type="text"]');
                if (inputEl && !inputEl.placeholder?.includes('章节')) continue; // skip chapter number
                if (inputEl) return inputEl;
            }
        }
        return null;
    });
    if (labelInput && labelInput.asElement()) {
        const el = labelInput;
        await el.click();
        await el.evaluate(e => e.value = '');
        await el.type(title, { delay: 20 });
        log('info', 'title filled via label');
        return;
    }

    log('warn', 'title input not found');
}

/**
 * 填写章节正文（contenteditable 编辑器）。
 */
async function fillContent(page, content) {
    // 优先 contenteditable
    const editor = await page.$('[contenteditable="true"]').catch(() => null);
    if (editor) {
        await editor.click();
        await sleep(200);

        // 清空：选中全部内容后删除
        await page.keyboard.down('Control');
        await page.keyboard.press('KeyA');
        await page.keyboard.up('Control');
        await page.keyboard.press('Backspace');
        await sleep(200);

        // 使用 document.execCommand('insertText') 插入文本，
        // 富文本编辑器的内部模型能正确感知这种插入方式，字数统计正常
        await editor.evaluate((el, text) => {
            el.focus();
            // 确保编辑器内容为空
            if (el.textContent && el.textContent.trim()) {
                el.innerHTML = '';
            }
            const ok = document.execCommand('insertText', false, text);
            if (!ok) {
                // fallback: 直接用 textContent（比 innerHTML 更安全）
                el.textContent = text;
            }
            // 触发完整事件链让编辑器内部模型同步
            el.dispatchEvent(new Event('input', { bubbles: true }));
            el.dispatchEvent(new Event('change', { bubbles: true }));
        }, content);

        await sleep(300);

        log('info', 'content filled via contenteditable (execCommand)');
        return;
    }

    // 回退：textarea
    const textarea = await page.$('textarea').catch(() => null);
    if (textarea) {
        await textarea.click();
        await textarea.evaluate(el => el.value = '');
        await textarea.type(content, { delay: 5 });
        log('info', 'content filled via textarea');
        return;
    }

    log('warn', 'content editor not found');
}

/**
 * 导航到章节编辑器页面。
 * "新建章节" 按钮被包裹在 target=_blank 的 <a> 标签中，不能直接点击，
 * 需要提取 href 后直接跳转。
 */
async function navigateToChapterEditor(page) {
    try {
        // 尝试从"新建章节"链接提取 href
        const url = await page.evaluate(() => {
            const links = document.querySelectorAll('a');
            for (const a of links) {
                const text = (a.textContent || '').trim();
                if (text === '新建章节' || text === '创建章节') {
                    return a.href;
                }
            }
            return null;
        });

        if (url) {
            log('info', 'navigating to chapter editor', { url });
            await page.goto(url, { waitUntil: 'domcontentloaded', timeout: CONFIG.NAVIGATION_TIMEOUT });
            return;
        }

        // 回退：尝试从页面 URL 找到 novel ID 拼接 publish URL
        const publishUrl = await page.evaluate(() => {
            const currentUrl = window.location.href;
            const match = currentUrl.match(/\/writer\/(\d+)/);
            if (match) {
                return '/main/writer/' + match[1] + '/publish/?enter_from=newchapter';
            }
            return null;
        });
        if (publishUrl) {
            log('info', 'navigating to chapter editor (fallback)', { url: publishUrl });
            await page.goto(CONFIG.BASE_URL + publishUrl, { waitUntil: 'domcontentloaded', timeout: CONFIG.NAVIGATION_TIMEOUT });
            return;
        }

        // 兜底：点击"新建章节"并等待新页面弹出
        log('warn', 'trying click 新建章节 as last resort');
        await clickButton(page, ['新建章节', '创建章节']);
    } catch (e) {
        log('warn', 'navigateToChapterEditor error', { error: e.message });
        throw new Error('无法进入章节编辑器: ' + e.message);
    }
}

/**
 * 检查是否已进入章节编辑器页面。（有标题输入框或正文编辑器即为编辑页）
 */
async function isOnEditorPage(page) {
    try {
        return await page.evaluate(() => {
            // 有 contenteditable 编辑器
            if (document.querySelector('[contenteditable="true"]')) return true;
            // 有标题输入框
            const inputs = document.querySelectorAll('input[placeholder*="章节"], input[placeholder*="标题"], input[name*="title"]');
            if (inputs.length > 0) return true;
            // 有 textarea
            if (document.querySelector('textarea')) return true;
            return false;
        });
    } catch {
        return false;
    }
}

/**
 * 关闭页面上的所有可见弹窗/引导提示/遮罩层。
 *
 * 策略：自动检测页面上所有可见的浮层容器（modal/dialog/popup/overlay），
 * 依次尝试关闭——先找关闭按钮（×），再找确认文字按钮（我知道了/知道了/好的/关闭），
 * 最后点击遮罩层。不使用硬编码的弹窗内容文本。
 *
 * 重试 3 次，确保同时出现的多个弹窗全部关闭。
 *
 * 不会点击"立即生成"/"去生成"等触发动作的按钮——这些是正向操作而非关闭。
 */
async function dismissPopups(page) {
    const DANGEROUS_BUTTONS = new Set([
        '立即生成', '去生成', '确认发布', '发布', '生成',
        '提交', '保存', '下一步', '确定删除', '删除', '解除',
        '取消', '确定',
    ]);
    const DISMISS_TEXTS = ['我知道了', '知道了', '好的', '关闭', '确定', '跳过', '否'];
    const CLOSE_TEXTS = ['×', '✕', 'x', 'X', '关闭'];

    for (let retry = 0; retry < 3; retry++) {
        let anyClicked = false;
        try {
            const result = await page.evaluate(
                ({ dangerousSet, dismissTexts, closeTexts }) => {

                // ---- 查找所有可见浮层容器 ----
                const overlaySelectors = [
                    // 按 z-index 从高到低排序找可见元素
                    '[style*="z-index"]',
                    '[class*="modal"]', '[class*="Modal"]',
                    '[class*="dialog"]', '[class*="Dialog"]',
                    '[class*="popup"]', '[class*="Popup"]',
                    '[class*="overlay"]', '[class*="Overlay"]',
                    '[class*="mask"]', '[class*="Mask"]',
                    '[class*="backdrop"]', '[class*="Backdrop"]',
                    '[class*="toast"]', '[class*="Toast"]',
                    '[class*="notice"]', '[class*="Notice"]',
                    '[class*="tooltip"]', '[class*="Tooltip"]',
                    '[class*="guide"]', '[class*="Guide"]',
                    '[class*="drawer"]', '[class*="Drawer"]',
                    '[class*="confirm"]', '[class*="Confirm"]',
                    '[role="dialog"]', '[role="alertdialog"]',
                    '.byte-popconfirm-wrapper',
                    '.chapter-setting-wrap-tips-typo-btn',
                ];

                // 收集所有候选弹窗容器（去重）
                const seen = new Set();
                const popups = [];
                for (const sel of overlaySelectors) {
                    const els = document.querySelectorAll(sel);
                    for (const el of els) {
                        if (el.offsetParent === null || el.offsetParent === undefined) continue;
                        if (el.offsetWidth === 0 && el.offsetHeight === 0) continue;
                        if (seen.has(el)) continue;
                        seen.add(el);
                        // 收缩到弹窗根容器（往上找到 fixed/absolute + z-index 的最近祖先）
                        let root = el;
                        let p = el.parentElement;
                        while (p && p !== document.body && p !== document.documentElement) {
                            const style = getComputedStyle(p);
                            if ((style.position === 'fixed' || style.position === 'absolute')
                                && parseInt(style.zIndex) > 0) {
                                root = p;
                            }
                            p = p.parentElement;
                        }
                        if (seen.has(root)) continue;
                        seen.add(root);
                        popups.push(root);
                    }
                }

                if (popups.length === 0) return null;

                // 按 z-index 从高到低排序（先处理顶层弹窗）
                popups.sort((a, b) => {
                    const za = parseInt(getComputedStyle(a).zIndex) || 0;
                    const zb = parseInt(getComputedStyle(b).zIndex) || 0;
                    return zb - za;
                });

                // ---- 对每个弹窗尝试关闭 ----
                for (const popup of popups) {
                    if (popup.offsetParent === null) continue; // 已被前面的关闭操作移除

                    // 策略 1：查找关闭图标按钮（CSS class / aria-label / SVG）
                    const closeByIcon = tryCloseByIcon(popup);
                    if (closeByIcon) return closeByIcon;

                    // 策略 2：在弹窗内查找 ×/✕/关闭 文字元素
                    const closeByText = tryCloseByText(popup, closeTexts);
                    if (closeByText) return closeByText;

                    // 策略 3：查找确认/取消类文字按钮（我知道了/知道了/好的/关闭/确定/跳过/否）
                    const dismissByText = tryClickDismissText(popup, dismissTexts, dangerousSet);
                    if (dismissByText) return dismissByText;

                    // 策略 4：点击遮罩层（overlay/mask 自身）
                    const clickOverlay = tryClickOverlay(popup);
                    if (clickOverlay) return clickOverlay;

                    // 策略 5：点击弹窗外部以关闭（某些弹窗点击外部即可关闭）
                    if (popup.parentElement && popup.parentElement !== document.body) {
                        popup.parentElement.click();
                        return 'click_popup_parent';
                    }
                }

                return null;

                // ---- 内部辅助函数 ----

                function isVisible(el) {
                    return el && el.offsetParent !== null && el.offsetWidth > 0 && el.offsetHeight > 0;
                }

                function isInsidePopup(el, popupRoot) {
                    return popupRoot.contains(el);
                }

                function isDangerous(text) {
                    const t = (text || '').trim();
                    return dangerousSet.includes(t) || t === '' || t.length > 10;
                }

                function tryCloseByIcon(popup) {
                    const selectors = [
                        '[class*="close"]', '[class*="Close"]',
                        '[class*="dismiss"]', '[class*="Dismiss"]',
                        '[class*="cancel"]', '[class*="Cancel"]',
                        '[aria-label*="关闭"]', '[aria-label*="close"]',
                        '[aria-label*="Close"]', '[aria-label*="取消"]',
                        'svg[class*="close"]', 'svg[class*="Close"]',
                        'i[class*="close"]', 'i[class*="Close"]',
                        'button[class*="icon"]',
                    ];
                    for (const sel of selectors) {
                        const el = popup.querySelector(sel);
                        if (el && isVisible(el) && !isDangerous(el.textContent)) {
                            el.click();
                            return 'icon_' + sel.slice(0, 25);
                        }
                    }
                    // 也搜索全局（某些关闭按钮在 popup 外但在弹窗层内）
                    for (const sel of selectors) {
                        const el = document.querySelector(sel);
                        if (el && isVisible(el) && !isDangerous(el.textContent)) {
                            el.click();
                            return 'icon_global_' + sel.slice(0, 25);
                        }
                    }
                    return null;
                }

                function tryCloseByText(popup, texts) {
                    const candidates = popup.querySelectorAll('button, span, div, a, [role="button"], i, svg');
                    for (const el of candidates) {
                        const t = (el.textContent || '').trim();
                        if (texts.includes(t) && isVisible(el)) {
                            el.click();
                            return 'txt_' + t;
                        }
                    }
                    return null;
                }

                function tryClickDismissText(popup, dismissTexts, dangerousSet) {
                    const buttons = popup.querySelectorAll('button, span[role="button"], [role="button"], a.btn, div[class*="btn"]');
                    for (const el of buttons) {
                        if (!isVisible(el)) continue;
                        const t = (el.textContent || '').trim();
                        if (isDangerous(t)) continue;
                        if (dismissTexts.includes(t)) {
                            el.click();
                            return 'btn_' + t;
                        }
                    }
                    return null;
                }

                function tryClickOverlay(popup) {
                    const overlaySel = '[class*="overlay"], [class*="Overlay"], [class*="mask"], [class*="Mask"], [class*="backdrop"], [class*="Backdrop"]';
                    const overlays = popup.querySelectorAll(overlaySel);
                    for (const ov of overlays) {
                        if (isVisible(ov)) {
                            ov.click();
                            return 'overlay_child';
                        }
                    }
                    if (popup.matches(overlaySel) && isVisible(popup)) {
                        popup.click();
                        return 'overlay_self';
                    }
                    const allOverlays = document.querySelectorAll(overlaySel);
                    for (const ov of allOverlays) {
                        if (isVisible(ov) && ov.textContent.trim().length < 50) {
                            ov.click();
                            return 'overlay_global';
                        }
                    }
                    return null;
                }
            }, { dangerousSet: Array.from(DANGEROUS_BUTTONS), dismissTexts: DISMISS_TEXTS, closeTexts: CLOSE_TEXTS });

            if (result) {
                log('info', 'popup dismissed', { action: result });
                anyClicked = true;
                await sleep(1500);
            } else {
                break; // 没有可关闭的弹窗
            }
        } catch (e) {
            log('warn', 'dismissPopups error', { error: e.message });
            break;
        }
    }
}

/**
 * 处理"请选择内容检测方式"弹窗。
 * 新版番茄在点击"下一步"后会弹出内容检测方式选择：
 *   - 全面检测（有次数限制）
 *   - 基础检测（不限次数）
 * 此函数点击"仅基础检测"快速跳过，节省时间和检测配额。
 *
 * @returns {Promise<boolean>} 是否找到并处理了弹窗
 */
async function handleContentDetectionDialog(page) {
    try {
        for (let i = 0; i < 5; i++) {
            await sleep(1000);
            const result = await page.evaluate(() => {
                const modals = document.querySelectorAll('.global-confirm-modal, .check-modal-confirm, [class*="check-modal"]');
                for (const modal of modals) {
                    if (modal.offsetParent === null) continue;
                    const text = (modal.textContent || '').trim();
                    if (text.includes('请选择内容检测方式') || text.includes('检测方式')) {
                        // 优先点击"仅基础检测"快速跳过
                        const btns = modal.querySelectorAll('button');
                        for (const btn of btns) {
                            const t = (btn.textContent || '').trim();
                            if (t === '仅基础检测' || t === '基础检测') {
                                btn.click();
                                return '仅基础检测';
                            }
                        }
                        // 回退：点击"全面检测"
                        for (const btn of btns) {
                            const t = (btn.textContent || '').trim();
                            if (t === '全面检测') {
                                btn.click();
                                return '全面检测';
                            }
                        }
                        // 回退：关闭弹窗
                        const closeBtn = modal.querySelector('.arco-modal-close-icon, [class*="close"]');
                        if (closeBtn && closeBtn.offsetParent !== null) {
                            closeBtn.click();
                            return 'close_icon';
                        }
                        return 'found_no_btn';
                    }
                }
                return null;
            });

            if (result) {
                log('info', 'content detection dialog handled', { action: result });
                await sleep(2000);
                return true;
            }
        }
        return false;
    } catch (e) {
        log('warn', 'handleContentDetectionDialog error', { error: e.message });
        return false;
    }
}

async function handleTypoCheckAndNext(page) {
    try {
        // === 阶段1：处理"内容风险检测"弹窗 ===
        log('info', 'checking for risk detection dialog');
        const riskHandled = await handleRiskDetectionDialog(page);
        if (riskHandled) {
            log('info', 'risk detection dialog handled');
        }

        // === 阶段2：等待错别字检测完成 ===
        log('info', 'waiting for typo check to complete');
        let foundIgnore = false;

        for (let i = 0; i < 8; i++) {
            await sleep(2000);

            // 检查是否有"忽略全部"按钮
            const hasIgnore = await page.evaluate(() => {
                const btns = document.querySelectorAll('button, span, div');
                for (const btn of btns) {
                    if (btn.textContent && btn.textContent.trim() === '忽略全部') {
                        btn.click();
                        return true;
                    }
                }
                return false;
            });

            if (hasIgnore) {
                log('info', 'clicked 忽略全部');
                foundIgnore = true;
                await sleep(1500);
                try {
                    log('info', 'clicking next step after ignoring typos');
                    await clickButton(page, ['下一步']);
                    await sleep(2000);
                } catch (e) {
                    log('warn', 'next step click after typo ignore failed', { error: e.message });
                }
                return true;
            }

            // 检查错别字检测是否完成
            const checkDone = await page.evaluate(() => {
                const body = document.body.innerText;
                if (body.includes('暂无错别字') || body.includes('智能纠错 (0)')) {
                    return true;
                }
                if (body.includes('重新检测') && !body.includes('检测中')) {
                    return true;
                }
                return false;
            });

            if (checkDone) {
                log('info', 'typo check completed, no errors to ignore');
                return false;
            }
        }

        log('warn', 'typo check timed out, proceeding anyway');
        return false;
    } catch (e) {
        log('warn', 'handleTypoCheckAndNext error', { error: e.message });
        return false;
    }
}

/**
 * 处理"内容风险检测"弹窗：点击"确定"跳过。
 * @returns {Promise<boolean>} 是否找到并处理了弹窗
 */
async function handleRiskDetectionDialog(page) {
    try {
        // 等待弹窗出现（最多 5 秒）
        for (let i = 0; i < 5; i++) {
            await sleep(1000);

            // 检查是否有风险检测弹窗
            const detected = await page.evaluate(() => {
                const body = document.body.innerText || '';
                if (body.includes('是否进行风险内容检测') || body.includes('风险检测')) {
                    // 优先点"基础检测"或"仅基础检测"
                    const btns = document.querySelectorAll('button, span, div');
                    for (const btn of btns) {
                        const t = (btn.textContent || '').trim();
                        if (t === '仅基础检测' || t === '基础检测') {
                            btn.click();
                            return t;
                        }
                    }
                    // 回退：点击"取消"跳过（更省时间）
                    for (const btn of btns) {
                        const t = (btn.textContent || '').trim();
                        if (t === '取消') {
                            btn.click();
                            return '取消';
                        }
                    }
                    return 'found_no_cancel';
                }
                // 检查 global-confirm-modal
                const modal = document.querySelector('.global-confirm-modal');
                if (modal && modal.offsetParent !== null) {
                    // Modal is visible
                    const btns = modal.querySelectorAll('button');
                    for (const btn of btns) {
                        const t = (btn.textContent || '').trim();
                        if (t === '取消') {
                            btn.click();
                            return '取消_modal';
                        }
                        if (t === '确定') {
                            btn.click();
                            return '确定_modal';
                        }
                    }
                    // 按钮不匹配预期，尝试处理其他类型的确认弹窗
                    const btnsAll = modal.querySelectorAll('button');
                    for (const btn of btnsAll) {
                        const t = (btn.textContent || '').trim();
                        if (t === '仅基础检测' || t === '基础检测') {
                            btn.click();
                            return '仅基础检测';
                        }
                        if (t === '全面检测') {
                            btn.click();
                            return '全面检测';
                        }
                        if (t === '我知道了' || t === '知道了' || t === '好的') {
                            btn.click();
                            return t;
                        }
                    }
                    // 关闭弹窗作为最后手段
                    const closeBtn = modal.querySelector('.arco-modal-close-icon, [class*="close"]');
                    if (closeBtn && closeBtn.offsetParent !== null) {
                        closeBtn.click();
                        return 'close_icon';
                    }
                    return 'modal_no_btn';
                }
                return null;
            });

            if (detected) {
                log('info', 'risk detection dialog result', { action: detected });
                await sleep(1000);
                await sleep(2000);
                return true;
            }
        }
        return false;
    } catch (e) {
        log('warn', 'handleRiskDetectionDialog error', { error: e.message });
        return false;
    }
}

/**
 * 从页面 URL 中提取 chapter ID。
 */
async function getChapterIdFromURL(page) {
    try {
        await sleep(2000);
        const chapterId = await page.evaluate(() => {
            const url = window.location.href;

            // 方法1: chapter/数字 或 chapter_id=数字
            let m = url.match(/chapter\/(\d+)/) || url.match(/chapter_id=(\d+)/);
            if (m) return m[1];

            // 方法2: writer/{workId}/publish/{chapterId}（新版番茄编辑页）
            m = url.match(/\/publish\/(\d+)/);
            if (m) return m[1];

            // 方法3: zone/article/数字
            m = url.match(/zone\/article\/(\d+)/);
            if (m) return m[1];

            // 方法4: 从页面链接中查找
            const links = document.querySelectorAll('a');
            for (const link of links) {
                const href = link.getAttribute('href') || '';
                const cm = href.match(/chapter\/(\d+)/) || href.match(/\/publish\/(\d+)/);
                if (cm) return cm[1];
            }

            // 方法5: data属性
            const inputs = document.querySelectorAll('[data-chapter-id], [data-chapterid]');
            for (const el of inputs) {
                const id = el.getAttribute('data-chapter-id') || el.getAttribute('data-chapterid');
                if (id && /^\d+$/.test(id)) return id;
            }

            return null;
        });
        return chapterId;
    } catch {
        return null;
    }
}

/**
 * 处理发布设置页面：选择分卷、AI 选项、确认定时发布关闭，最后点击确认发布。
 */
/**
 * 处理出现在发布设置页面的"是否进行内容风险检测？"弹窗。
 * 与 handleRiskDetectionDialog 不同，此弹窗出现在进入发布设置页后，
 * 必须优先点击"取消"跳过，否则会阻塞后续发布流程。
 *
 * @returns {Promise<boolean>} 是否找到并处理了弹窗
 */
async function handleRiskDetectionAtSettings(page) {
    try {
        for (let i = 0; i < 5; i++) {
            await sleep(1000);
            const result = await page.evaluate(() => {
                const modals = document.querySelectorAll('.global-confirm-modal, [class*="modal"]');
                for (const modal of modals) {
                    if (modal.offsetParent === null) continue;
                    const text = (modal.textContent || '').trim();
                    if (text.includes('是否进行风险内容检测') || text.includes('风险检测')) {
                        // 优先点"基础检测"或"仅基础检测"
                        const btns = modal.querySelectorAll('button');
                        for (const btn of btns) {
                            const t = (btn.textContent || '').trim();
                            if (t === '仅基础检测' || t === '基础检测') {
                                btn.click();
                                return t;
                            }
                        }
                        // 回退：点击"取消"跳过
                        for (const btn of btns) {
                            const t = (btn.textContent || '').trim();
                            if (t === '取消') {
                                btn.click();
                                return '取消';
                            }
                        }
                        // 回退：点击关闭图标
                        const closeBtn = modal.querySelector('.arco-modal-close-icon, [class*="close"]');
                        if (closeBtn && closeBtn.offsetParent !== null) {
                            closeBtn.click();
                            return 'close_icon';
                        }
                        return 'found_no_btn';
                    }
                }
                return null;
            });
            if (result) {
                log('info', 'risk detection at settings handled', { action: result });
                await sleep(2000);
                return true;
            }
        }
        return false;
    } catch (e) {
        log('warn', 'handleRiskDetectionAtSettings error', { error: e.message });
        return false;
    }
}

async function handlePublishSettings(page) {
    await sleep(2000);

    // 调试：保存设置页面截图
    try {
        await page.screenshot({ path: '/tmp/fanqie_settings.png', fullPage: false });
    } catch (e) {
        log('warn', 'settings debug save failed', { error: e.message });
    }

    // 0) 显式处理"是否进行内容风险检测？"弹窗 → 点"取消"跳过
    // 此弹窗可能在进入发布设置页时出现，必须先处理否则阻塞后续操作
    await handleRiskDetectionAtSettings(page);

    // 0.5) 关闭其他引导弹窗（"知道了"等）
    await dismissPopups(page);
    await sleep(500);

    // 1) 选择分卷："第一卷：默认" 或 "第一卷"
    await selectVolume(page);

    // 2) AI 选项：选"否"
    await selectAIOption(page);

    // 2.5) 处理 AI 内容声明（新版番茄新增）
    await handleAIContentDeclaration(page);

    // 3) 确认定时发布处于关闭状态
    await ensureTimerPublishOff(page);
}

/**
 * 处理"发布提示"确认弹窗。
 * 在点击"确认发布"/"提交"后，番茄小说会弹出确认对话框：
 *   "检测到你还有错别字未修改，是否确定提交？"
 * 此函数自动点击"提交"按钮以完成发布。
 *
 * @returns {Promise<boolean>} 是否找到并处理了弹窗
 */
async function handlePublishConfirmModal(page) {
    try {
        const result = await page.evaluate(() => {
            // 查找所有可见的弹窗/对话框
            const modalSelectors = [
                '.publish-modal-confirm', '.global-confirm-modal', '.auto-editor-error-modal',
                '[class*="publish-modal"]', '[class*="confirm-modal"]', '[class*="confirm-dialog"]',
                '[class*="dialog"]', '[class*="modal"]', '[class*="popup"]', '[class*="toast"]',
                '[class*="notification"]', '[role="dialog"]', '[role="alertdialog"]',
            ];
            const seen = new Set();
            for (const sel of modalSelectors) {
                try {
                    const elms = document.querySelectorAll(sel);
                    for (const modal of elms) {
                        if (modal.offsetParent === null) continue;
                        const key = modal.className + '|' + (modal.id || '');
                        if (seen.has(key)) continue;
                        seen.add(key);

                        const modalText = (modal.textContent || '').trim();

                        // 匹配发布确认相关文案
                        const confirmPatterns = [
                            '发布提示', '是否确定提交', '确定发布', '确认发布', '确认提交',
                            '是否发布', '提交作品', '提交章节', '发布章节', '立即发布',
                            '确认操作', '操作确认', '提示', '温馨提示',
                        ];
                        let matched = false;
                        for (const p of confirmPatterns) {
                            if (modalText.includes(p)) { matched = true; break; }
                        }
                        if (!matched) continue;

                        // 按优先级点击确认按钮
                        const confirmBtns = ['提交', '确认', '确定', '确认提交', '确认发布', '发布', '知道了', '确定提交', '是'];
                        for (const label of confirmBtns) {
                            const btns = modal.querySelectorAll('button, a[class*="btn"], span[class*="btn"], div[class*="btn"]');
                            for (const btn of btns) {
                                const t = (btn.textContent || '').trim();
                                if (t === label) {
                                    btn.click();
                                    return 'clicked:' + label;
                                }
                            }
                        }
                        return 'found_no_btn:' + modalText.substring(0, 40);
                    }
                } catch (_) {}
            }
            return null;
        });

        if (result) {
            log('info', 'publish confirm modal handled', { action: result });
            await sleep(2000);
            return true;
        }
        return false;
    } catch (e) {
        log('warn', 'handlePublishConfirmModal error', { error: e.message });
        return false;
    }
}

/**
 * 处理 AI 内容声明选项。
 * 新版番茄平台要求声明章节是否包含 AI 生成内容。
 */
async function handleAIContentDeclaration(page) {
    try {
        const result = await page.evaluate(() => {
            // 找 AI 内容声明区域
            const declareEl = document.querySelector('[class*="ai-content-declare"], [class*="ai-declare"]');
            if (!declareEl || declareEl.offsetParent === null) return 'not_found';

            // 找到"否"或"不含"选项并点击
            const labels = declareEl.querySelectorAll('label, span, div, button');
            for (const el of labels) {
                const t = (el.textContent || '').trim();
                if (t === '否' || t === '不含' || t === '无' || t === '不包含') {
                    el.click();
                    return 'clicked_' + t;
                }
            }

            // 尝试点击 radio/checkbox 中非 AI 的选项
            const radios = declareEl.querySelectorAll('input[type="radio"], input[type="checkbox"]');
            // 通常第一个是"是"第二个是"否"
            if (radios.length >= 2) {
                radios[1].click();
                radios[1].dispatchEvent(new Event('change', { bubbles: true }));
                return 'radio_second';
            }

            return 'no_option';
        });

        if (result && result !== 'not_found' && result !== 'no_option') {
            log('info', 'AI content declaration handled', { result });
            await sleep(500);
        }
    } catch (e) {
        log('warn', 'handleAIContentDeclaration error', { error: e.message });
    }
}

/**
 * 在发布设置页面选择分卷。
 * 优先精确匹配"第一卷：默认"，回退到"第一卷"。
 */
async function selectVolume(page) {
    try {
        const clicked = await page.evaluate(() => {
            const candidates = document.querySelectorAll('option, label, span, div, li, button, input, select');
            // 先精确匹配"第一卷：默认"
            for (const el of candidates) {
                const text = (el.textContent || '').trim();
                if (text === '第一卷：默认' || text === '第一卷:默认') {
                    el.click();
                    if (el.tagName === 'OPTION') {
                        // 如果是 select 的 option，需要同时触发 change 事件
                        const select = el.closest('select');
                        if (select) select.dispatchEvent(new Event('change', { bubbles: true }));
                    }
                    return '第一卷：默认';
                }
            }
            // 回退："第一卷"
            for (const el of candidates) {
                const text = (el.textContent || '').trim();
                if (text === '第一卷') {
                    el.click();
                    if (el.tagName === 'OPTION') {
                        const select = el.closest('select');
                        if (select) select.dispatchEvent(new Event('change', { bubbles: true }));
                    }
                    return '第一卷';
                }
            }
            return null;
        });
        if (clicked) {
            log('info', 'volume selected', { volume: clicked });
        } else {
            log('warn', 'volume selector not found, skipping');
        }
    } catch (e) {
        log('warn', 'selectVolume error', { error: e.message });
    }
}

/**
 * 在发布设置页面选择是否使用 AI。
 * 默认选"否"。
 */
async function selectAIOption(page) {
    try {
        const clicked = await page.evaluate(() => {
            // 先尝试定位包含"AI"文字的容器，再在容器内找"否"
            const allElements = document.querySelectorAll('label, span, div, button, input');
            for (const el of allElements) {
                const text = (el.textContent || '').trim();
                if (text.includes('AI') || text.includes('是否使用')) {
                    const container = el.closest('div, form, fieldset, section, [class*="group"]');
                    if (container) {
                        const children = container.querySelectorAll('label, span, div, button, input, [role="radio"], [class*="radio"]');
                        for (const child of children) {
                            const childText = (child.textContent || '').trim();
                            if (childText === '否') {
                                child.click();
                                return true;
                            }
                        }
                    }
                }
            }
            // 回退：在整个页面中找"否"（临近 AI 相关文字的）
            for (const el of allElements) {
                const text = (el.textContent || '').trim();
                if (text === '否') {
                    // 检查附近的文字是否包含 AI
                    const parent = el.closest('div, label, form, fieldset');
                    if (parent && (parent.textContent.includes('AI') || parent.textContent.includes('是否使用'))) {
                        el.click();
                        return true;
                    }
                }
            }
            return false;
        });
        if (clicked) {
            log('info', 'AI option selected: 否');
        } else {
            log('warn', 'AI option selector not found, skipping');
        }
    } catch (e) {
        log('warn', 'selectAIOption error', { error: e.message });
    }
}

/**
 * 确认定时发布处于关闭状态。
 * 如果定时开关是开着的，则点击关闭。
 */
async function ensureTimerPublishOff(page) {
    try {
        const result = await page.evaluate(() => {
            // 查找定时发布相关的开关/toggle
            const switches = document.querySelectorAll('[role="switch"], [class*="switch"], [class*="toggle"], input[type="checkbox"], [class*="Switch"]');
            for (const el of switches) {
                // 找到包含"定时"文字的父容器
                let parent = el.closest('div, label, form, li, span, [class*="row"], [class*="item"]');
                if (parent && parent.textContent && parent.textContent.includes('定时')) {
                    const isOn = el.checked === true
                        || el.getAttribute('aria-checked') === 'true'
                        || (el.className && (el.className.includes('active') || el.className.includes('checked') || el.className.includes('on')));
                    if (isOn) {
                        el.click();
                        return 'turned_off';
                    }
                    return 'already_off';
                }
            }
            return 'not_found';
        });
        log('info', 'timer publish status', { result });
    } catch (e) {
        log('warn', 'ensureTimerPublishOff error', { error: e.message });
    }
}

function escapeHtml(str) {
    return str
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
}

// ======================== 启动 ========================

main().catch(err => {
    log('error', 'unhandled error', { error: err.message, stack: err.stack });
    output({ success: false, error: err.message });
});
