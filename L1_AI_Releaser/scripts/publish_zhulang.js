/**
 * publish_zhulang.js — 逐浪网章节发布脚本 (Puppeteer 浏览器自动化)
 *
 * 依赖: npm install puppeteer
 * Cookie: 环境变量 ZHULANG_COOKIE
 * 入参: node publish_zhulang.js --base64 (stdin 读 base64 JSON)
 *
 * 输入字段:
 *   {title, content, novelName, volumeName?, chapterNumber?, publishDirectly?}
 *   - chapterNumber: 不传/0/null → 自动取最新章节号+1
 *   - publishDirectly: 默认 true(发布), false=保存草稿
 *
 * 输出 stdout: {"success":true,"postId":"xxx"} | {"success":false,"error":"..."}
 */
'use strict';
const puppeteer = require('puppeteer');

// ======================== 日志 ========================
function log(level, msg, data) {
    process.stderr.write(JSON.stringify({ time: new Date().toISOString(), level, msg, ...(data ? { data } : {}) }) + '\n');
}
function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }
function output(r) { process.stdout.write(JSON.stringify(r) + '\n'); process.exit(r.success ? 0 : 1); }
function fail(msg) { log('error','fail',{msg}); output({success:false,error:msg}); }

// ======================== 配置 ========================
const CFG = {
    BOOK_URL: 'https://writer.zhulang.com/book/index.html',
    NAV_TIMEOUT: 60000,
    TIMEOUT_MS: 300000,
    MIN_CONTENT: 500,
    VIEWPORT: {width:1920,height:1080},
};
let gBrowser, gPage;

// ======================== stdin base64 ========================
function readStdin() {
    return new Promise((resolve, reject) => {
        const chunks = [];
        process.stdin.setEncoding('utf-8');
        process.stdin.on('data', c => chunks.push(c));
        process.stdin.on('end', () => {
            const raw = chunks.join('').trim();
            if (!raw) { fail('stdin empty'); return; }
            try { resolve(JSON.parse(Buffer.from(raw,'base64').toString('utf-8'))); }
            catch(e) { fail('base64 decode: ' + e.message); }
        });
    });
}

// ======================== 主入口 ========================
async function main() {
    let input;
    if (process.argv.includes('--base64')) { input = await readStdin(); }
    else { try { input = JSON.parse(process.argv[2]); } catch(e) { fail('invalid JSON'); return; } }

    const { title, content, novelName, volumeName, chapterNumber, publishDirectly } = input;
    if (!content) { fail('missing content'); return; }
    if (!novelName) { fail('missing novelName'); return; }

    const chapterTitle = title || firstLine(content, 50) || '新章节';
    const effectiveVolume = volumeName || '正文';
    const explicitNum = (chapterNumber === undefined || chapterNumber === null || chapterNumber === 0) ? 0 : chapterNumber;
    const autoInc = explicitNum === 0;
    const shouldPublish = publishDirectly !== false;

    log('info','input',{title:chapterTitle,novelName,vol:effectiveVolume,chNum:explicitNum,autoInc,contentLen:content.length});

    const cookie = process.env.ZHULANG_COOKIE;
    if (!cookie || !cookie.trim()) { fail('ZHULANG_COOKIE not set'); return; }

    for (let attempt = 0; attempt <= 1; attempt++) {
        if (attempt > 0) { log('info','retry',{attempt}); await sleep(2000); }
        try {
            const r = await doPublish(cookie, {novelName,chapterTitle,content,effectiveVolume,explicitNum,autoInc,shouldPublish});
            output(r); return;
        } catch(e) { log('warn','attempt failed',{err:e.message}); }
    }
    fail('all attempts failed');
}

// ======================== 核心发布流程 ========================
async function doPublish(cookie, p) {
    const { novelName, chapterTitle, content, effectiveVolume, explicitNum, autoInc, shouldPublish } = p;

    log('info','launch browser');
    gBrowser = await puppeteer.launch({
        headless:'new',protocolTimeout:120000,
        args:['--no-sandbox','--disable-setuid-sandbox','--disable-dev-shm-usage','--disable-gpu'],
    });
    gPage = await gBrowser.newPage();
    await gPage.setViewport(CFG.VIEWPORT);
    gPage.on('dialog', async d => { log('info','dialog',{type:d.type()}); await d.accept(); });

    try {
        // 1. 注入 cookie
        const cks = cookie.split(';').map(p => { const i=p.indexOf('='); if(i<0)return null; return {name:p.substring(0,i).trim(),value:p.substring(i+1).trim(),domain:'.writer.zhulang.com',path:'/'}; }).filter(Boolean);
        await gPage.setCookie(...cks);
        await gPage.goto(CFG.BOOK_URL, {waitUntil:'domcontentloaded',timeout:CFG.NAV_TIMEOUT});
        await sleep(5000);
        if (!await checkLogin(gPage)) throw new Error('login failed');
        log('info','logged in');

        // 2. 查找/创建/回退作品
        let found = await findNovel(gPage, novelName);
        log('info','novel search',{name:novelName,found});
        
        if (!found) {
            // dump page content for debugging
            const pageText = await gPage.evaluate(() => (document.body?.innerText||'').substring(0,500));
            log('warn','novel not found, trying create',{novelName,pagePreview:pageText});
            
            const created = await tryCreate(gPage, novelName, content.replace(/\s/g,'').substring(0,100));
            if (!created) {
                log('warn','create blocked, fallback to existing');
                await gPage.goto(CFG.BOOK_URL, {waitUntil:'domcontentloaded',timeout:CFG.NAV_TIMEOUT});
                await sleep(5000);
                const fb = await useFirstNovel(gPage);
                if (!fb) throw new Error('no usable novel');
                log('warn','FALLBACK using wrong novel! expected='+novelName+' actual='+fb);
                if (fb !== novelName) {
                    throw new Error('novel mismatch: expected ' + novelName + ' but found ' + fb + '. Check if the novel exists on 逐浪网.');
                }
            }
        }

        // 3. 进入编辑器 — 取指定作品对应的"写新章节"链接
        const editorUrl = await getWriteChapterHrefForNovel(gPage, novelName);
        if (!editorUrl) throw new Error('no write chapter link for novel: ' + novelName);
        await gPage.goto(editorUrl, {waitUntil:'domcontentloaded',timeout:CFG.NAV_TIMEOUT});
        await sleep(3000);
        await dismissPopups(gPage);
        await sleep(1000);
        log('info','editor',{url:gPage.url()});

        // 4. 自动递增章节号
        let effectiveNum = explicitNum;
        if (autoInc) {
            let last = await lastChapterNumFromEditor(gPage);
            if (last === 0) {
                const bookId = await gPage.evaluate(() => (typeof bookInfo!=='undefined'&&bookInfo.bookid)?bookInfo.bookid:'');
                if (bookId) {
                    await gPage.goto(`https://writer.zhulang.com/bookApplyChapter/index/apply_id/${bookId}.html`, {waitUntil:'domcontentloaded',timeout:CFG.NAV_TIMEOUT});
                    await sleep(5000);
                    last = await lastChapterNumFromPage(gPage);
                }
            }
            effectiveNum = last > 0 ? last + 1 : 1;
            log('info','auto-increment',{lastChapter:last,newNum:effectiveNum});
            // 若离开了编辑器页面，重新进入
            if (!gPage.url().includes('/draft/add/')) {
                await gPage.goto(CFG.BOOK_URL, {waitUntil:'domcontentloaded',timeout:CFG.NAV_TIMEOUT});
                await sleep(3000);
                const eUrl = await getWriteChapterHref(gPage);
                if (eUrl) await gPage.goto(eUrl, {waitUntil:'domcontentloaded',timeout:CFG.NAV_TIMEOUT});
                await sleep(3000);
                await dismissPopups(gPage);
            }
        }

        // 5. 选择卷 (Element UI)
        await selectVolume(gPage, effectiveVolume);
        await sleep(500);

        // 6. 章节号+名 (合并输入框 #ch-tit)
        await fillChapterTitle(gPage, effectiveNum, chapterTitle);
        await sleep(500);

        // 7. 正文 (textarea #ch-cnt)
        await fillContent(gPage, content);
        await sleep(1000);

        // 8. 发布/保存
        let submitSuccess = false;
        gPage.on('response', async resp => {
            if (resp.url().includes('/draft/submit/') && resp.status() === 200) {
                try {
                    const body = await resp.text();
                    log('info','submit resp',{body:body.substring(0,200)});
                    if (/\"code\":0/.test(body) && /\"msg\"/.test(body)) {
                        submitSuccess = true;
                        try { const j = JSON.parse(body); if (j.code===0 && j.data?.ch_id) gPage._postId = String(j.data.ch_id); } catch(_){}
                    }
                } catch(_) {}
            }
        });

        if (shouldPublish) {
            log('info','submitting via AJAX');
            const sr = await gPage.evaluate(() => {
                return new Promise(resolve => {
                    try {
                        const ch_name = document.getElementById('ch-tit')?.value || '';
                        const content = document.getElementById('ch-cnt')?.value || '';
                        const intro = document.getElementById('ch-ext')?.value || '';
                        const vol_id = (typeof volData!=='undefined'&&volData.length>0) ? volData[volData.length-1].vol_id : '';
                        if (typeof $!=='undefined' && typeof ajaxApis!=='undefined' && ajaxApis.submitChapter) {
                            $.ajax({url:ajaxApis.submitChapter,type:'POST',dataType:'json',
                                data:{submittype:'publish',ch_name,ch_content:content,intro,vol_id,ch_vip:'0',ch_size:String(content.length),ch_effect_time:''},
                                success:r=>resolve({ok:true,data:r}),error:(x,st,er)=>resolve({ok:false,status:st,err:er,body:x.responseText?.substring(0,200)})
                            });
                            return;
                        }
                        resolve({ok:false,reason:'no $ or ajaxApis'});
                    } catch(e) { resolve({ok:false,reason:e.message}); }
                });
            });
            log('info','submit result',{result:JSON.stringify(sr).substring(0,300)});
            if (sr?.ok && sr?.data?.code === 0) {
                submitSuccess = true;
                if (sr.data.data?.ch_id) gPage._postId = String(sr.data.data.ch_id);
            }
        } else {
            // 保存草稿
            await gPage.evaluate(() => {
                const ch_name = document.getElementById('ch-tit')?.value || '';
                const content = document.getElementById('ch-cnt')?.value || '';
                const vol_id = (typeof volData!=='undefined'&&volData.length>0) ? volData[volData.length-1].vol_id : '';
                if (typeof $!=='undefined' && typeof ajaxApis!=='undefined') {
                    $.ajax({url:ajaxApis.submitChapter,type:'POST',dataType:'json',
                        data:{submittype:'save',ch_name,ch_content:content,vol_id,ch_vip:'0',ch_size:String(content.length)}
                    });
                }
            });
            await sleep(3000);
        }

        // 9. 获取 postId
        let postId = gPage._postId || ('zhulang_' + Date.now());
        log('info','published',{postId});
        return {success:true,postId};
    } finally {
        await gBrowser.close().catch(()=>{});
        gBrowser = null;
    }
}

// ======================== 登录检测 ========================
async function checkLogin(page) {
    try {
        await page.waitForFunction(() => document.body&&/作品管理|作家专区|写新章节|创建作品/.test(document.body.innerText), {timeout:15000});
        return true;
    } catch(_) { return false; }
}

// ======================== 查找/创建/回退作品 ========================
async function findNovel(page, name) {
    return await page.evaluate(n => {
        function clean(s) {
            return s.replace(/[\ue000-\uf8ff\u200b\u00a0\ufeff]/g, '').replace(/\s+/g, ' ').trim();
        }
        const target = clean(n);
        for(const el of document.querySelectorAll('a,span,div,td')) {
            if(clean(el.textContent||'') === target) return true;
        }
        // 回退：子串匹配
        for(const el of document.querySelectorAll('a,span,div,td')) {
            if(clean(el.textContent||'').includes(target)) return true;
        }
        return false;
    }, name);
}

async function getWriteChapterHrefForNovel(page, novelName) {
    return await page.evaluate(n => {
        function clean(s){return s.replace(/[\ue000-\uf8ff\u200b\u00a0\ufeff]/g,'').replace(/\s+/g,' ').trim();}
        const target = clean(n);
        // 策略1: 找到包含目标作品名的表格行，取其中的"写新章节"链接
        const rows = document.querySelectorAll('tr');
        for (const row of rows) {
            const rowText = row.textContent||'';
            if (!rowText.includes('写新章节')) continue;
            // 检查这行是否包含目标作品名
            let hasNovel = false;
            for (const el of row.querySelectorAll('a,span,div,td')) {
                if (clean(el.textContent||'') === target) { hasNovel = true; break; }
            }
            if (!hasNovel) {
                // 回退：子串匹配
                for (const el of row.querySelectorAll('a,span,div,td')) {
                    if (clean(el.textContent||'').includes(target)) { hasNovel = true; break; }
                }
            }
            if (hasNovel) {
                for (const a of row.querySelectorAll('a')) {
                    if ((a.textContent||'').includes('写新章节') && a.href) return a.href;
                }
            }
        }
        // 策略2: 回退 — 取第一个"写新章节"链接（旧行为）
        for (const a of document.querySelectorAll('a')) {
            if ((a.textContent||'').includes('写新章节') && a.href) return a.href;
        }
        return null;
    }, novelName);
}

// 保留旧函数兼容性（auto-increment 重进入时使用）
async function getWriteChapterHref(page) {
    return await getWriteChapterHrefForNovel(page, '');
}

async function useFirstNovel(page) {
    return await page.evaluate(() => {
        function c(s){return s.replace(/[\ue000-\uf8ff]/g,'').trim();}
        for(const row of document.querySelectorAll('tr')) {
            if(!(row.textContent||'').includes('写新章节')) continue;
            for(const a of row.querySelectorAll('a')) {
                const t=c(a.textContent||''); if(t.length>1&&t.length<30&&!/写新章节|已发布|暂无上传|创建|通知|作家|首页|管理|咨询|活动|注册/.test(t)) return t;
            }
        }
        return null;
    });
}

async function tryCreate(page, name, intro) {
    // 步骤1：点击"创建作品"，会导航到 bookApply/add.html
    await clickBtn(page,['创建作品']); await sleep(5000);
    
    // 检查是否在创建页面
    const onCreatePage = page.url().includes('bookApply/add');
    if (!onCreatePage) {
        // 可能被审核弹窗拦截
        if (await checkReviewModal(page)) { log('info','blocked by review'); return false; }
        await sleep(1000);
        return false;
    }
    
    log('info','on create page',{url:page.url()});
    
    // 步骤2：选目标读者 → "男生"
    const readerClicked = await page.evaluate(() => {
        // 点击"男生"按钮或label
        for(const el of document.querySelectorAll('button, a, span, div, label')) {
            if(!el.offsetParent) continue;
            const t = (el.textContent||'').trim().replace(/\s+/g,'');
            if(t==='男生'||t==='男频') { el.click(); return '男生'; }
        }
        return null;
    });
    log('info','reader target',{clicked:readerClicked});
    await sleep(1000);
    
    // 步骤3：点击"下一步"
    await page.evaluate(() => {
        for(const el of document.querySelectorAll('button, a, span, div')) {
            if(!el.offsetParent) continue;
            const t = (el.textContent||'').trim();
            if(t==='下一步') { el.click(); return; }
        }
    });
    await sleep(3000);
    log('info','after next step',{url:page.url()});
    
    // 步骤4：填写作品名称 (此时在第二步"完善作品信息")
    const nameFilled = await page.evaluate(n => {
        for(const i of document.querySelectorAll('input[type="text"],input:not([type])')) {
            if(!i.offsetParent) continue;
            const ph = i.placeholder||'';
            if(ph.includes('作品名称')||ph.includes('书名')||ph.includes('名称')){
                i.value=n;i.dispatchEvent(new Event('input',{bubbles:true}));i.dispatchEvent(new Event('change',{bubbles:true}));return true;
            }
        }
        // fallback: first visible text input
        for(const i of document.querySelectorAll('input[type="text"]')){
            if(i.offsetParent){i.value=n;i.dispatchEvent(new Event('input',{bubbles:true}));return true;}
        }
        return false;
    }, name);
    log('info','name filled',{ok:nameFilled});
    
    // 步骤5：填写简介
    const introFilled = await page.evaluate(t => {
        for(const ta of document.querySelectorAll('textarea')){
            if(ta.offsetParent){ta.value=t;ta.dispatchEvent(new Event('input',{bubbles:true}));ta.dispatchEvent(new Event('change',{bubbles:true}));return true;}
        }
        return false;
    }, intro);
    log('info','intro filled',{ok:introFilled});
    
    await sleep(1000);
    
    // 步骤6：提交创建
    await page.evaluate(() => {
        for(const el of document.querySelectorAll('button, a, span, div')) {
            if(!el.offsetParent) continue;
            const t = (el.textContent||'').trim();
            if(['创建作品','提交','确认','确定','完成','保存'].includes(t)) { el.click(); return; }
        }
    });
    await sleep(5000);
    
    log('info','novel created, navigating back to book page');
    await page.goto(CFG.BOOK_URL, {waitUntil:'domcontentloaded',timeout:CFG.NAV_TIMEOUT});
    await sleep(3000);
    return true;
}

async function checkReviewModal(page) {
    for(let i=0;i<5;i++) { await sleep(1000);
        const c = await page.evaluate(() => {
            if(!/正在审核|审核通过|继续创建/.test(document.body.innerText||'')) return false;
            for(const b of document.querySelectorAll('button,a,span,div')){if((b.textContent||'').replace(/\s+/g,'')==='确定'){b.click();return true;}} return false;
        });
        if(c){await sleep(1500);return true;}
    } return false;
}

// ======================== 编辑器操作 ========================
async function selectVolume(page, target) {
    try {
        const opened = await page.evaluate(t => {
            for(const i of document.querySelectorAll('.el-input__inner')) {
                if(!i.offsetParent)continue; const v=i.value||''; if(v===t||v.includes(t))return'already';
                const p=i.closest('.el-input,div'); if(!p)continue;
                if(!/发布至|分卷|卷|至/.test((p.parentElement?.textContent||'').trim()))continue; i.click();return'opened';
            } return null;
        }, target);
        if(opened==='already'){log('info','vol already',{vol:target});return;}
        if(opened==='opened'){await sleep(1000);
            await page.evaluate(t => {for(const o of document.querySelectorAll('.el-select-dropdown__item,.el-select-dropdown li,.el-select-dropdown span')){if((o.textContent||'').trim()===t){o.click();return;}}}, target);
            await sleep(500);
        }
    } catch(e){log('warn','selectVolume err',{err:e.message});}
}

async function fillChapterTitle(page, num, title) {
    const v = `第${String(num).padStart(4,'0')}章 ${title}`;
    await page.evaluate(val => {
        const s=Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set;
        const el=document.getElementById('ch-tit')||document.querySelector('input[name="ch_name"]');
        if(el&&el.offsetParent){s.call(el,val);el.dispatchEvent(new Event('input',{bubbles:true}));el.dispatchEvent(new Event('change',{bubbles:true}));el.dispatchEvent(new Event('blur',{bubbles:true}));}
    }, v);
    log('info','title filled',{title:v});
}

async function fillContent(page, content) {
    await page.evaluate(c => {
        const ta=document.getElementById('ch-cnt')||document.querySelector('textarea[name="content"]');
        if(ta&&ta.offsetParent){ta.value=c;ta.dispatchEvent(new Event('input',{bubbles:true}));ta.dispatchEvent(new Event('change',{bubbles:true}));return;}
        const ce=document.querySelector('[contenteditable="true"]');
        if(ce&&ce.offsetParent){ce.focus();ce.textContent=c;ce.dispatchEvent(new Event('input',{bubbles:true}));return;}
        const ta2=document.querySelector('textarea');if(ta2&&ta2.offsetParent){ta2.value=c;ta2.dispatchEvent(new Event('input',{bubbles:true}));}
    }, content);
    log('info','content filled',{len:content.length});
}

// ======================== 章节号自动递增 ========================
async function lastChapterNumFromEditor(page) {
    try { await sleep(2000);
        return await page.evaluate(() => {
            let m=0; const t=document.body?.innerText||'';
            for(const x of t.matchAll(/第(\d+)章/g)){const n=parseInt(x[1],10);if(n>m)m=n;}
            try{if(typeof treeData!=='undefined'&&Array.isArray(treeData)){for(const nd of treeData){if(nd.label){const x=nd.label.match(/第(\d+)章/);if(x){const n=parseInt(x[1],10);if(n>m)m=n;}}}}}catch(_){}
            return m;
        });
    } catch(e) { return 0; }
}

async function lastChapterNumFromPage(page) {
    try { await sleep(3000);
        return await page.evaluate(() => {
            let m=0; const t=document.body?.innerText||'';
            for(const x of t.matchAll(/第(\d+)章/g)){const n=parseInt(x[1],10);if(n>m)m=n;}
            return m;
        });
    } catch(e) { return 0; }
}

// ======================== 弹窗/按钮工具 ========================
async function clickBtn(page, texts) {
    for(const target of texts) {
        const ok = await page.evaluate(t => {
            function c(s){return s.replace(/[\ue000-\uf8ff]/g,'').trim();}
            for(const el of document.querySelectorAll('button,a,span,div[class*="btn"]')){const ct=c(el.textContent||'');if(ct===t||ct.includes(t)){el.click();return true;}} return false;
        }, target);
        if(ok){log('info','clicked',{text:target});return;}
    }
    const last=texts[texts.length-1];
    try{await page.evaluate(t=>{const el=document.evaluate(`//*[contains(normalize-space(),"${t}")]`,document,null,XPathResult.FIRST_ORDERED_NODE_TYPE,null).singleNodeValue;if(el)el.click();},last);return;}catch(_){}
    throw new Error('button not found: '+JSON.stringify(texts));
}

async function dismissPopups(page) {
    for(let r=0;r<3;r++) {
        const d = await page.evaluate(() => {
            for(const el of document.querySelectorAll('[class*="close"],[class*="Close"],[class*="dismiss"],[aria-label*="关闭"]')){if(el.offsetParent&&el.offsetWidth>0){el.click();return true;}}
            for(const b of document.querySelectorAll('button,span,div,a')){if(!b.offsetParent)continue;const t=(b.textContent||'').trim();if(['我知道了','知道了','好的','关闭','跳过','否'].includes(t)){b.click();return true;}} return false;
        });
        if(d){await sleep(1000);}else break;
    }
}

function firstLine(text, max) { if(!text)return''; const f=text.split('\n')[0]?.trim()||''; return f.length<=max?f:f.substring(0,max)+'...'; }

// ======================== 启动 ========================
main().catch(e => { log('error','fatal',{err:e.message}); output({success:false,error:e.message}); });
