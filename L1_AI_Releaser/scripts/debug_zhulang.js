/**
 * debug_zhulang.js — 逐浪网作家专区诊断脚本
 *
 * 用途：探测逐浪网页面的真实 DOM 结构，收集 input/button/modal 等元素的
 * 信息，帮助调试和调整 publish_zhulang.js 的自动化逻辑。
 *
 * 运行方式：
 *   1. 将 Cookie 写入 /tmp/zhulang_cookie.txt（key=value; 格式）
 *   2. node debug_zhulang.js
 *
 * 输出：
 *   - 控制台打印页面结构信息
 *   - 截图保存到 /tmp/zhulang_debug_*.png
 *   - 关键元素信息保存到 /tmp/zhulang_debug.json
 */

'use strict';

const fs = require('fs');
const puppeteer = require('puppeteer');

let gPage = null;

async function screenshot(label) {
    const path = '/tmp/zhulang_' + (label || 'debug') + '.png';
    try {
        await gPage.screenshot({ path, fullPage: true });
        console.log('[screenshot]', path);
    } catch (e) {
        console.log('[screenshot_fail]', label, e.message);
    }
}

async function dumpPage(label) {
    console.log('\n==========', label, '==========');

    let title = '';
    let url = '';
    try {
        title = await gPage.title();
        url = gPage.url();
    } catch (_) {}
    console.log('  Title:', title);
    console.log('  URL:', url);

    const pageInfo = await gPage.evaluate(() => {
        const result = {};

        function isVisible(el) {
            return el && el.offsetParent !== null && el.offsetWidth > 0 && el.offsetHeight > 0;
        }

        // 1) 所有可见按钮
        const btns = document.querySelectorAll('button, a[href], [role="button"]');
        result.visibleButtons = [];
        const seenBtn = new Set();
        for (const btn of btns) {
            if (!isVisible(btn)) continue;
            const t = (btn.textContent || '').trim().replace(/\s+/g, ' ');
            if (!t || t.length > 50 || seenBtn.has(t)) continue;
            seenBtn.add(t);
            result.visibleButtons.push({ text: t, style: getComputedStyle(btn).display });
        }
        result.visibleButtonCount = result.visibleButtons.length;

        // 2) 所有可见 input / textarea
        const inputs = document.querySelectorAll('input, textarea, select');
        result.visibleInputs = [];
        for (const inp of inputs) {
            if (!isVisible(inp)) continue;
            result.visibleInputs.push({
                tag: inp.tagName,
                type: inp.type || '',
                name: inp.name || '',
                id: inp.id || '',
                placeholder: inp.placeholder || '',
                className: (inp.className || '').substring(0, 80),
                value: inp.value ? inp.value.substring(0, 40) : '',
            });
        }
        result.visibleInputCount = result.visibleInputs.length;

        // 3) 所有可见 select 及 option
        result.selects = [];
        for (const sel of selectors) {
            if (!isVisible(sel)) continue;
            const opts = [];
            for (const opt of sel.querySelectorAll('option')) {
                opts.push(opt.textContent.trim());
            }
            result.selects.push({ name: sel.name, id: sel.id, options: opts.slice(0, 20) });
        }

        // 4) 可见的 contenteditable
        result.contenteditableCount = document.querySelectorAll('[contenteditable="true"]');
        // Filter visible
        let ce = 0;
        document.querySelectorAll('[contenteditable="true"]').forEach(el => { if (isVisible(el)) ce++; });
        result.contenteditableVisibleCount = ce;

        // 5) 可见的 modal/dialog/弹窗
        const modalSels = [
            '[role="dialog"]', '[role="alertdialog"]',
            '[class*="modal"]', '[class*="Modal"]',
            '[class*="dialog"]', '[class*="Dialog"]',
            '[class*="popup"]', '[class*="Popup"]',
            '[class*="overlay"]', '[class*="Overlay"]',
            '[class*="drawer"]', '[class*="Drawer"]',
            '.layui-layer',
        ];
        result.visibleModals = [];
        for (const sel of modalSels) {
            const elms = document.querySelectorAll(sel);
            for (const el of elms) {
                if (isVisible(el)) {
                    result.visibleModals.push({
                        sel,
                        text: (el.textContent || '').trim().substring(0, 200),
                        className: (el.className || '').substring(0, 80),
                    });
                }
            }
        }
        result.visibleModalCount = result.visibleModals.length;

        // 6) iframe
        result.iframes = [];
        for (const iframe of document.querySelectorAll('iframe')) {
            result.iframes.push({ id: iframe.id, src: iframe.src, visible: isVisible(iframe) });
        }

        // 7) 页面 body 文字（前500字）
        result.bodyText = (document.body ? document.body.innerText : '').substring(0, 500);

        return result;
    });

    console.log('  visibleButtons:', pageInfo.visibleButtonCount);
    pageInfo.visibleButtons.slice(0, 20).forEach(b => console.log('    -', b.text));
    if (pageInfo.visibleButtonCount > 20) console.log('    ... and', pageInfo.visibleButtonCount - 20, 'more');

    console.log('  visibleInputs:', pageInfo.visibleInputCount);
    pageInfo.visibleInputs.forEach(b => console.log('    -', JSON.stringify(b)));

    console.log('  selects:', pageInfo.selects.length);
    pageInfo.selects.forEach(s => console.log('    -', s.name, s.id, s.options));

    console.log('  contenteditable (visible):', pageInfo.contenteditableVisibleCount);
    console.log('  visibleModals:', pageInfo.visibleModalCount);
    pageInfo.visibleModals.forEach(m => console.log('    -', m.sel, m.className, m.text.substring(0, 80)));
    console.log('  iframes:', pageInfo.iframes.length);
    pageInfo.iframes.forEach(f => console.log('    -', f.id, f.src, f.visible));
    console.log('  bodyText (first 500):', pageInfo.bodyText);

    fs.writeFileSync('/tmp/zhulang_dump_' + (label || 'debug') + '.json', JSON.stringify(pageInfo, null, 2));
}

// ======================== 主流程 ========================

(async () => {
    let cookieStr;
    try {
        cookieStr = fs.readFileSync('/tmp/zhulang_cookie.txt', 'utf8').trim();
        console.log('[cookie] loaded, len=' + cookieStr.length);
    } catch (e) {
        console.error('ERROR: 请先将 Cookie 写入 /tmp/zhulang_cookie.txt');
        console.error('  格式: key1=value1; key2=value2');
        process.exit(1);
    }

    const browser = await puppeteer.launch({
        headless: 'new',
        args: [
            '--no-sandbox',
            '--disable-setuid-sandbox',
            '--disable-dev-shm-usage',
            '--disable-gpu',
        ],
    });

    gPage = await browser.newPage();
    await gPage.setViewport({ width: 1920, height: 1080 });

    // 自动接受 alert/confirm/prompt
    gPage.on('dialog', async dialog => {
        console.log('[dialog]', dialog.type(), dialog.message());
        await dialog.accept();
    });

    // ========== Step 1: 注入 Cookie + 打开作家专区 ==========
    const domain = 'writer.zhulang.com';
    const cookies = cookieStr.split(';').map(pair => {
        const idx = pair.indexOf('=');
        if (idx === -1) return null;
        return {
            name: pair.substring(0, idx).trim(),
            value: pair.substring(idx + 1).trim(),
            domain: '.' + domain,
            path: '/',
        };
    }).filter(Boolean);
    console.log('[cookie] parsed ' + cookies.length + ' cookies:', cookies.map(c => c.name));

    await gPage.setCookie(...cookies);
    await gPage.goto('https://writer.zhulang.com/', {
        waitUntil: 'networkidle2',
        timeout: 30000,
    });
    console.log('[nav] loaded:', await gPage.title());
    await new Promise(r => setTimeout(r, 3000));

    await screenshot('01_writer_home');
    await dumpPage('01_writer_home');

    // ========== Step 2: 点"作品管理" ==========
    console.log('\n>>> Step 2: 点击 作品管理');
    const btnClicked2 = await gPage.evaluate(() => {
        const keywords = ['作品管理', '我的作品', '作品', '管理作品'];
        const elms = document.querySelectorAll('button, a, span, div, li');
        for (const el of elms) {
            if (el.offsetParent === null) continue;
            const t = (el.textContent || '').trim();
            if (keywords.includes(t)) { el.click(); return t; }
        }
        return null;
    });
    if (btnClicked2) {
        console.log('[click] 作品管理 button clicked:', btnClicked2);
    } else {
        console.log('[click] 作品管理 button NOT found, trying fuzzy match');
        await gPage.evaluate(() => {
            const elms = document.querySelectorAll('button, a, span, div, li');
            for (const el of elms) {
                if (el.offsetParent === null) continue;
                const t = (el.textContent || '').trim();
                if (t.includes('作品') || t.includes('管理')) {
                    el.click();
                    return;
                }
            }
        });
    }
    await new Promise(r => setTimeout(r, 3000));
    await screenshot('02_after_works_mgmt');
    await dumpPage('02_after_works_mgmt');

    // ========== Step 3: 找"创建作品"按钮 ==========
    console.log('\n>>> Step 3: 点击 创建作品');
    const btnClicked3 = await gPage.evaluate(() => {
        const keywords = ['创建作品', '新建作品', '写新书', '创建新书', '新建'];
        const elms = document.querySelectorAll('button, a, span, div, li');
        for (const el of elms) {
            if (el.offsetParent === null) continue;
            const t = (el.textContent || '').trim();
            if (keywords.includes(t)) { el.click(); return t; }
        }
        return null;
    });
    if (btnClicked3) {
        console.log('[click] create button:', btnClicked3);
    } else {
        console.log('[click] create button NOT found');
    }
    await new Promise(r => setTimeout(r, 3000));
    await screenshot('03_after_create_click');
    await dumpPage('03_after_create_click');

    // ========== Step 4: 观察弹窗，找"男生"/"男频"按钮 ==========
    console.log('\n>>> Step 4: 找目标读者弹窗（男生/男频）');
    const btnClicked4 = await gPage.evaluate(() => {
        const keywords = ['男生', '男', '男频', '男性向'];
        const elms = document.querySelectorAll('button, a, span, div, label, li');
        for (const el of elms) {
            if (el.offsetParent === null) continue;
            const t = (el.textContent || '').trim();
            if (keywords.includes(t)) { el.click(); return t; }
        }
        return null;
    });
    if (btnClicked4) {
        console.log('[click] reader target clicked:', btnClicked4);
    } else {
        console.log('[click] reader target NOT found');
    }
    await new Promise(r => setTimeout(r, 2000));

    // 点"下一步"
    const btnClickedNext = await gPage.evaluate(() => {
        const keywords = ['下一步', '继续', '确认', '确定'];
        const elms = document.querySelectorAll('button, a, span, div');
        for (const el of elms) {
            if (el.offsetParent === null) continue;
            const t = (el.textContent || '').trim();
            if (keywords.includes(t)) { el.click(); return t; }
        }
        return null;
    });
    if (btnClickedNext) {
        console.log('[click] next step:', btnClickedNext);
    } else {
        console.log('[click] next step button NOT found');
    }
    await new Promise(r => setTimeout(r, 3000));
    await screenshot('04_after_reader_target');
    await dumpPage('04_after_reader_target');

    // ========== Step 5: 填新书表单 ==========
    console.log('\n>>> Step 5: 填写新书信息');

    // 填书名
    const nameFilled = await gPage.evaluate(() => {
        const placeholders = ['作品名称', '书名', '名称', '标题', '作品名'];
        const inputs = document.querySelectorAll('input[type="text"], input:not([type])');
        for (const inp of inputs) {
            if (inp.offsetParent === null) continue;
            const ph = inp.placeholder || '';
            for (const p of placeholders) {
                if (ph.includes(p)) {
                    inp.value = '穿越之小龙虾传奇';
                    inp.dispatchEvent(new Event('input', { bubbles: true }));
                    return 'placeholder_' + p;
                }
            }
        }
        // fallback: first visible text input
        for (const inp of inputs) {
            if (inp.offsetParent === null) continue;
            inp.value = '穿越之小龙虾传奇';
            inp.dispatchEvent(new Event('input', { bubbles: true }));
            return 'first_input';
        }
        return null;
    });
    console.log('[fill] novel name:', nameFilled);

    // 填简介
    const introFilled = await gPage.evaluate(() => {
        const tareas = document.querySelectorAll('textarea');
        for (const ta of tareas) {
            if (ta.offsetParent === null) continue;
            ta.value = '江湖路远，侠义永存。一个少年手持长剑，踏上未知的征途。他将在纷争与恩怨中历练成长，书写属于他自己的传奇篇章。武林浩劫将至，唯有意志坚定之人才能守护这片大地。';
            ta.dispatchEvent(new Event('input', { bubbles: true }));
            return 'textarea';
        }
        return null;
    });
    console.log('[fill] intro:', introFilled);

    await new Promise(r => setTimeout(r, 1000));
    await screenshot('05_after_form_fill');
    await dumpPage('05_after_form_fill');

    // ========== Step 6: 点提交/创建作品 ==========
    console.log('\n>>> Step 6: 提交创建');
    const submitClicked = await gPage.evaluate(() => {
        const keywords = ['创建作品', '提交', '确认', '确定', '保存', '下一步', '完成'];
        const elms = document.querySelectorAll('button, a, span');
        for (const el of elms) {
            if (el.offsetParent === null) continue;
            const t = (el.textContent || '').trim();
            if (keywords.includes(t)) { el.click(); return t; }
        }
        return null;
    });
    if (submitClicked) {
        console.log('[click] submit:', submitClicked);
    } else {
        console.log('[click] submit button NOT found');
    }
    await new Promise(r => setTimeout(r, 5000));
    await screenshot('06_after_submit');
    await dumpPage('06_after_submit');

    // ========== Step 7: 找"新建章节" ==========
    console.log('\n>>> Step 7: 找新建章节');
    const newChapterClicked = await gPage.evaluate(() => {
        const keywords = ['新建章节', '创建章节', '写新章节', '新增章节'];
        const elms = document.querySelectorAll('button, a, span, div');
        for (const el of elms) {
            if (el.offsetParent === null) continue;
            const t = (el.textContent || '').trim();
            if (keywords.includes(t)) { el.click(); return t; }
        }
        return null;
    });
    if (newChapterClicked) {
        console.log('[click] new chapter:', newChapterClicked);
    } else {
        console.log('[click] new chapter button NOT found - checking links');
        // 尝试从链接中获取 href 导航
        const href = await gPage.evaluate(() => {
            const links = document.querySelectorAll('a');
            for (const a of links) {
                const t = (a.textContent || '').trim();
                if (t.includes('新建章节') || t.includes('创建章节')) {
                    return a.href;
                }
            }
            return null;
        });
        if (href) {
            console.log('[nav] found href:', href);
            await gPage.goto(href, { waitUntil: 'networkidle2', timeout: 30000 });
        }
    }
    await new Promise(r => setTimeout(r, 3000));
    await screenshot('07_chapter_editor');
    await dumpPage('07_chapter_editor');

    // ========== Step 8: 填章节内容 ==========
    console.log('\n>>> Step 8: 填写章节');

    // 卷
    const volSelected = await gPage.evaluate(() => {
        const selects = document.querySelectorAll('select');
        for (const sel of selects) {
            if (sel.offsetParent === null) continue;
            const opts = sel.querySelectorAll('option');
            console.log('[select] options:', Array.from(opts).map(o => o.textContent.trim()));
            // 选第一个非默认的
            if (opts.length > 1) {
                opts[0].selected = true;
                sel.dispatchEvent(new Event('change', { bubbles: true }));
                return 'selected_' + opts[0].textContent.trim();
            }
        }
        return null;
    });
    console.log('[fill] volume:', volSelected);

    // 章节号+名
    const chapterFilled = await gPage.evaluate(() => {
        const inputs = document.querySelectorAll('input[type="text"], input:not([type])');
        // 策略: 第1个填章节号, 第2个填章节名
        let numFilled = false;
        for (const inp of inputs) {
            if (inp.offsetParent === null) continue;
            const ph = inp.placeholder || '';
            if (!numFilled && (ph.includes('章') || ph.includes('序号') || ph.includes('编号'))) {
                inp.value = '0001';
                inp.dispatchEvent(new Event('input', { bubbles: true }));
                numFilled = true;
                continue;
            }
            if (numFilled && ph && ph.length > 0) {
                inp.value = '第一章 初入江湖';
                inp.dispatchEvent(new Event('input', { bubbles: true }));
                return 'separate';
            }
        }
        return numFilled ? 'num_only' : null;
    });
    console.log('[fill] chapter number+title:', chapterFilled);

    // 正文
    const demoContent = '第一章 初入江湖\n\n王小龙睁开眼，发现自己穿越到了一个完全陌生的世界。周围是一片广袤的竹林，竹叶在风中沙沙作响。他的手中握着一本破旧的古籍，封面上写着"小龙虾烹饪秘籍"几个大字。\n\n这与他在地球上的职业——小龙虾餐厅大厨——形成了奇妙的呼应。正当他困惑之际，一道系统提示音在脑海中响起："叮！恭喜宿主激活美食江湖系统！"';
    const contentFilled = await gPage.evaluate((content) => {
        // contenteditable
        const ce = document.querySelector('[contenteditable="true"]');
        if (ce && ce.offsetParent !== null) {
            ce.focus();
            ce.textContent = content;
            ce.dispatchEvent(new Event('input', { bubbles: true }));
            return 'contenteditable';
        }
        // textarea
        const ta = document.querySelector('textarea');
        if (ta && ta.offsetParent !== null) {
            ta.value = content;
            ta.dispatchEvent(new Event('input', { bubbles: true }));
            return 'textarea';
        }
        return null;
    }, demoContent);
    console.log('[fill] content:', contentFilled);

    await new Promise(r => setTimeout(r, 1000));
    await screenshot('08_after_chapter_fill');
    await dumpPage('08_after_chapter_fill');

    // ========== 最终摘要 ==========
    console.log('\n========================================');
    console.log('  诊断完成！');
    console.log('  截图保存在 /tmp/zhulang_*.png');
    console.log('  结构信息保存在 /tmp/zhulang_dump_*.json');
    console.log('========================================\n');

    await browser.close();
})();
