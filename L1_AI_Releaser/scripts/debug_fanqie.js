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
    console.log('Page loaded:', await page.title());
    
    // Click "作品管理"
    const btns = await page.$$('button, a, span, div');
    for (const btn of btns) {
        try {
            const text = await btn.evaluate(el => el.textContent);
            if (text && text.includes('作品管理')) {
                await btn.click();
                console.log('Clicked: 作品管理');
                break;
            }
        } catch {}
    }
    await new Promise(r => setTimeout(r, 3000));
    
    // Click "创建新书"
    const btns2 = await page.$$('button, a, span, div');
    for (const btn of btns2) {
        try {
            const text = await btn.evaluate(el => el.textContent);
            if (text && (text.includes('创建新书') || text.includes('新建作品') || text.includes('创建作品'))) {
                await btn.click();
                console.log('Clicked:', text.trim());
                break;
            }
        } catch {}
    }
    await new Promise(r => setTimeout(r, 4000));
    
    // Dump all inputs on page
    const inputs = await page.$$('input');
    console.log(`Found ${inputs.length} input elements`);
    for (const inp of inputs) {
        try {
            const info = await inp.evaluate(el => ({
                type: el.type,
                placeholder: el.placeholder,
                name: el.name,
                id: el.id,
                className: el.className,
                visible: el.offsetParent !== null
            }));
            if (info.visible) {
                console.log('VISIBLE input:', JSON.stringify(info));
            }
        } catch {}
    }
    
    // Dump body text to understand page structure
    const bodyText = await page.evaluate(() => document.body.innerText.substring(0, 1000));
    console.log('Body text (first 1000):', bodyText);
    
    await page.screenshot({ path: '/tmp/fanqie_create.png', fullPage: true });
    console.log('Screenshot saved');
    
    await browser.close();
})();
