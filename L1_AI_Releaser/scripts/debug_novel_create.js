const puppeteer = require('puppeteer');
const cookie = require('fs').readFileSync('/tmp/fanqie_cookie.txt', 'utf8').trim();

(async () => {
    const browser = await puppeteer.launch({ headless: 'new', args: ['--no-sandbox', '--disable-setuid-sandbox'] });
    const page = await browser.newPage();
    
    const cookies = cookie.split('; ').map(c => {
        const [name, ...rest] = c.split('=');
        return { name, value: rest.join('='), domain: '.fanqienovel.com' };
    });
    await page.setCookie(...cookies);
    
    await page.goto('https://fanqienovel.com/main/writer/', { waitUntil: 'networkidle2', timeout: 15000 });
    console.log('Loaded:', await page.title());
    
    // Monitor navigation
    const navTargets = [];
    browser.on('targetcreated', async (target) => {
        if (target.type() === 'page') {
            const p = await target.page();
            const url = p.url();
            navTargets.push(url);
            console.log('New page/tab:', url);
        }
    });
    
    // Click "创建新书"
    const btns = await page.$$('button, a, span, div');
    let clicked = false;
    for (const btn of btns) {
        try {
            const text = await btn.evaluate(el => el.textContent);
            if (text && text.trim() === '创建新书') {
                console.log('Found create button');
                await btn.click();
                clicked = true;
                break;
            }
        } catch {}
    }
    
    if (!clicked) console.log('Create button not found!');
    
    await new Promise(r => setTimeout(r, 5000));
    
    console.log('Current URL:', page.url());
    console.log('New tabs/pages:', navTargets);
    
    // Check if modal/dialog appeared
    const modals = await page.$$('[role="dialog"], .modal, .drawer, .arco-modal, .ant-modal');
    console.log(`Found ${modals.length} modals/dialogs`);
    
    // Dump all inputs again
    const inputs = await page.$$('input, textarea');
    for (const inp of inputs) {
        try {
            const info = await inp.evaluate(el => ({
                tag: el.tagName,
                type: el.type,
                placeholder: el.placeholder,
                name: el.name,
                visible: el.offsetParent !== null
            }));
            if (info.visible && (info.type === 'text' || info.tag === 'TEXTAREA')) {
                console.log('Input:', JSON.stringify(info));
            }
        } catch {}
    }
    
    // Try clicking "创建作品" too
    const btns3 = await page.$$('button, a, span, div');
    for (const btn of btns3) {
        try {
            const text = await btn.evaluate(el => el.textContent);
            if (text && (text.includes('创建作品') || text.includes('写新书'))) {
                await btn.click();
                console.log('Clicked:', text.trim());
                break;
            }
        } catch {}
    }
    await new Promise(r => setTimeout(r, 3000));
    
    console.log('After second click URL:', page.url());
    
    // Dump inputs again
    const inputs2 = await page.$$('input, textarea');
    for (const inp of inputs2) {
        try {
            const info = await inp.evaluate(el => ({
                tag: el.tagName,
                type: el.type,
                placeholder: el.placeholder,
                name: el.name,
                visible: el.offsetParent !== null
            }));
            if (info.visible && (info.type === 'text' || info.tag === 'TEXTAREA')) {
                console.log('Input2:', JSON.stringify(info));
            }
        } catch {}
    }
    
    await page.screenshot({ path: '/tmp/fanqie_after_create.png', fullPage: true });
    console.log('Screenshot saved');
    
    await browser.close();
})();
