import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import { execFileSync } from 'node:child_process';

const root = path.resolve(import.meta.dirname, '..');
const pages = fs.readdirSync(root).filter(f => f.endsWith('.html'));
const voidTags = new Set('area base br col embed hr img input link meta param source track wbr'.split(' '));
const errors = [];
let scripts = 0;
for (const file of pages) {
    const html = fs.readFileSync(path.join(root,file),'utf8');
    const ids = [...html.matchAll(/\bid="([^"]+)"/g)].map(m=>m[1]);
    if (new Set(ids).size !== ids.length) errors.push(`${file}: duplicate IDs`);
    for (const match of html.matchAll(/\b(?:src|href)="([^"]+)"/g)) {
        const url = match[1];
        if (/^(?:https?:|mailto:|data:|javascript:)/.test(url)) continue;
        const [target,fragment] = url.split('#');
        const local = path.join(root,(target.split('?')[0] || file).replace(/^\//,''));
        if (!fs.existsSync(local)) errors.push(`${file}: missing ${url}`);
        else if (fragment && !fs.readFileSync(local,'utf8').includes(`id="${fragment}"`)) errors.push(`${file}: missing anchor ${url}`);
    }
    const stack = [];
    const markup = html.replace(/<!--[\s\S]*?-->/g,'').replace(/<(script|style)\b[^>]*>[\s\S]*?<\/\1>/gi,'');
    for(const m of markup.matchAll(/<(\/?)([a-z][\w-]*)\b[^>]*>/gi)) {
        const tag=m[2].toLowerCase();
        if(voidTags.has(tag)) continue;
        if(!m[1]) stack.push(tag);
        else if(stack.pop() !== tag) errors.push(`${file}: mismatched closing ${tag}`);
    }
    if(stack.length) errors.push(`${file}: unclosed ${stack.join(',')}`);
    for(const m of html.matchAll(/<script\b([^>]*)>([\s\S]*?)<\/script>/gi)) {
        if(!m[2].trim()) continue;
        const tmp=path.join(os.tmpdir(),`prophit-inline-${process.pid}.mjs`);
        try { fs.writeFileSync(tmp,m[2]); execFileSync(process.execPath,['--check',tmp],{stdio:'pipe'}); scripts++; }
        catch(error) { errors.push(`${file}: inline script syntax: ${error.message}`); }
        finally { fs.rmSync(tmp,{force:true}); }
    }
}
function walk(dir) {
    for(const entry of fs.readdirSync(dir,{withFileTypes:true})) {
        const file=path.join(dir,entry.name);
        if(entry.isDirectory()) walk(file);
        else if(file.endsWith('.js')) {
            try { execFileSync(process.execPath,['--check',file],{stdio:'pipe'}); scripts++; }
            catch(error) {errors.push(`${file}: ${error.message}`);}
            const source=fs.readFileSync(file,'utf8');
            for(const m of source.matchAll(/(?:from\s+|import\s*)['"](\.[^'"]+)['"]/g)) {
                if(!fs.existsSync(path.resolve(path.dirname(file),m[1]))) errors.push(`${file}: missing module ${m[1]}`);
            }
        }
    }
}
walk(path.join(root,'js'));
globalThis.window={location:{origin:'https://test.invalid'}};
const {escapeHTML,safeURL}=await import('data:text/javascript;base64,' + fs.readFileSync(path.join(root,'js/utils/escape.js')).toString('base64'));
assert.equal(escapeHTML(`<img src=x onerror="alert('x')">`),'&lt;img src=x onerror=&quot;alert(&#39;x&#39;)&quot;&gt;');
assert.equal(safeURL('javascript:alert(1)'),'');
assert.equal(safeURL('data:text/html,test'),'');
assert.equal(safeURL('https://example.com/path'),'https://example.com/path');
const {age,remaining}=await import('data:text/javascript;base64,' + fs.readFileSync(path.join(root,'js/utils/time.js')).toString('base64'));
const clock=Date.parse('2026-10-03T12:00:00Z');
assert.equal(remaining('2026-10-03T13:00:00Z',clock),'1 hr 0 min left');
assert.equal(remaining('2026-10-03T12:19:00Z',clock),'19 min left');
assert.equal(remaining('2026-10-03T11:00:00Z',clock),'Closed');
assert.equal(age('2026-10-03T11:00:00Z',clock),'1 hr ago');
assert.equal(age('invalid',clock),'Time unavailable');
const dataModule = file => 'data:text/javascript;base64,'+fs.readFileSync(path.join(root,file)).toString('base64');
const cardSource=fs.readFileSync(path.join(root,'js/components/market-card.js'),'utf8').replace('../utils/escape.js',dataModule('js/utils/escape.js')).replace('../utils/time.js',dataModule('js/utils/time.js'));
const {marketCard,marketStatus}=await import('data:text/javascript;base64,'+Buffer.from(cardSource).toString('base64'));
assert.equal(marketStatus({resolution_status:'Live',lock_time:'2026-10-03T11:00:00Z'},clock),'Locked');
assert.equal(marketStatus({resolution_status:'Voided',start_time:'2026-10-04T12:00:00Z'},clock),'Voided');
const card=marketCard({id:3,title:'<img src=x onerror=alert(1)>',description:'Real context',category:'Sports',resolution_status:'Paused',news_url:'javascript:alert(1)',entry_coins:10,payout:25,volume:0});
assert.ok(card.includes('&lt;img'));assert.ok(!card.includes('javascript:'));assert.ok(card.includes('Be the first to predict'));assert.ok(card.includes('10 Coins'));assert.ok(!card.includes('>LIVE<'));
const marketPage = fs.readFileSync(path.join(root,'market.html'),'utf8');
// API-rendered choices must retain their tones and expose an exclusive selection.
const predictionButtons = [];
let predictionFormHidden = true;
const marketContext = vm.createContext({
    document:{
        querySelector:()=>({innerHTML:''}),
        addEventListener:()=>{},
        createElement:()=>({dataset:{},attributes:{},listeners:{},setAttribute(name,value){this.attributes[name]=value;},addEventListener(name,fn){this.listeners[name]=fn;}}),
        querySelectorAll:()=>predictionButtons,
        getElementById:id=>id==='tradeForm' ? {classList:{remove:()=>{predictionFormHidden=false;}}} : null
    },
    window:{addEventListener:()=>{}}
});
vm.runInContext(fs.readFileSync(path.join(root,'js/pages/market.js'),'utf8').replace(/^import .*;\r?\n/gm,''),marketContext);
vm.runInContext("currentMarket = {prediction_type:'binary'}",marketContext);
const yesButton = marketContext.createPredictionButton('Yes');
const noButton = marketContext.createPredictionButton('No');
predictionButtons.push(yesButton,noButton);
assert.equal(yesButton.className,'btn btn-yes','API-rendered Yes must keep its green tone');
assert.equal(noButton.className,'btn btn-no','API-rendered No must keep its red tone');
assert.equal(yesButton.attributes['aria-pressed'],'false');
yesButton.listeners.click();
assert.equal(predictionFormHidden,false,'selecting an outcome must show the confirmation');
assert.equal(yesButton.attributes['aria-pressed'],'true');
assert.equal(noButton.attributes['aria-pressed'],'false');
assert.equal(marketContext.readTypedAnswer(),'Yes','confirmation must use the selected Yes outcome');
noButton.listeners.click();
assert.equal(yesButton.attributes['aria-pressed'],'false');
assert.equal(noButton.attributes['aria-pressed'],'true');
assert.equal(marketContext.readTypedAnswer(),'No','switching choices must update the submitted outcome');
assert.equal(marketContext.createPredictionButton('Buffalo Bills').className,'btn btn-outline','named outcomes must retain their labels');
assert.equal(marketContext.createPredictionButton('constructor').className,'btn btn-outline');
const previewButton = marketContext.createPredictionButton('Yes',true);
assert.equal(previewButton.disabled,true);
assert.equal(previewButton.listeners.click,undefined,'draft choices must not allow selection');
// A settlement received while the page is open must refresh the sidebar wallet.
const walletListeners = new Map();
const balanceNode = {textContent:'--'};
let actualBalance = 90;
const sidebarContext = vm.createContext({
    HTMLElement:class {getAttribute(){return null;} querySelector(){return balanceNode;}},
    URLSearchParams, document:{getElementById:()=>balanceNode},
    window:{location:{search:''},addEventListener:(name,fn)=>walletListeners.set(name,fn),removeEventListener:(name,fn)=>{if(walletListeners.get(name)===fn)walletListeners.delete(name);}},
    ApiClient:{isAuthenticated:()=>true,get:async()=>({points:actualBalance})},
    customElements:{define:(name,ctor)=>{sidebarContext.Sidebar=ctor;}}
});
vm.runInContext(fs.readFileSync(path.join(root,'js/components/sidebar.js'),'utf8').replace(/^import .*;\r?\n/gm,'').replace('export class','class'),sidebarContext);
const sidebar = new sidebarContext.Sidebar();
sidebar.connectedCallback(); await new Promise(resolve=>setImmediate(resolve));
assert.equal(balanceNode.textContent,90);
actualBalance=110; walletListeners.get('prophit-live')({detail:{event:'wallet_updated'}}); await new Promise(resolve=>setImmediate(resolve));
assert.equal(balanceNode.textContent,110,'settlement must update the sidebar balance');
sidebar.disconnectedCallback(); assert.equal(walletListeners.has('prophit-live'),false);
for (const fabricatedValue of ['55%', '142,500', '1,204']) {
    assert.equal(marketPage.includes(fabricatedValue), false, `market.html: fabricated statistic ${fabricatedValue}`);
}
assert.equal(errors.length,0,errors.join('\n'));
console.log(`PASS: ${pages.length} HTML pages, ${scripts} script syntax checks, local links/modules, HTML structure, IDs, and XSS escaping/URL regressions.`);
