import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import assert from 'node:assert/strict';
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
const marketPage = fs.readFileSync(path.join(root,'market.html'),'utf8');
for (const fabricatedValue of ['55%', '142,500', '1,204']) {
    assert.equal(marketPage.includes(fabricatedValue), false, `market.html: fabricated statistic ${fabricatedValue}`);
}
assert.equal(errors.length,0,errors.join('\n'));
console.log(`PASS: ${pages.length} HTML pages, ${scripts} script syntax checks, local links/modules, HTML structure, IDs, and XSS escaping/URL regressions.`);
