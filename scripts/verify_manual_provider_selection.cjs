// Exercise manual provider selection with synthetic identities and no CPA or model calls.
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const { createHash } = require('node:crypto');
const { once } = require('node:events');
const { spawn } = require('node:child_process');
const { createInterface } = require('node:readline');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || '/Users/ceng/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');

const shared = 'shared-model';
const account = { auth_id: 'fixture-file', auth_index: 'credential-index', provider: 'codex', email: 'research@example.test',
  runtime_status: 'active', effective_available: true, models: [shared, 'regular-only'] };
function provider(id, index, executor, name, models) {
  return { auth_id: id, auth_index: index, provider: executor, provider_name: name, models, is_provider: true,
    type: 'config_api_key', runtime_status: 'registered', effective_available: true, manual_disabled: false, quality_paused: false };
}
const northOne = provider('north-route-one', 'north-01', 'north-executor', '北辰云', [shared, 'north-first']);
const northTwo = provider('north-route-two', 'north-02', 'north-executor', '北辰云', ['north-second', shared]);
const west = provider('west-route', 'west-03', 'west-executor', '西岭云', [shared, 'west-only']);
const question = { id: 'logic-candy', title: '糖果逻辑题', kind: 'logic', prompt: '共有多少颗糖？', answer: '21' };
const out = process.env.UI_OUTPUT;
const assets = ['index.html','style.css','app.js','view.js','login.js','client.js'];

async function test() {
  const fixture = spawn(process.execPath,[path.join(__dirname,'login_ui_fixture.cjs')],{stdio:['ignore','pipe','inherit']});
  const ended = once(fixture,'exit');
  let browser;
  try {
    const [line] = await Promise.race([once(createInterface({input:fixture.stdout}),'line'),ended.then(()=>{throw Error('Fixture unavailable');})]);
    const info = JSON.parse(line), url = new URL(info.embedded_url);
    assert.equal(info.pid,fixture.pid); assert.equal(url.hostname,'127.0.0.1'); assert.notEqual(url.port,'8317');
    browser = await chromium.launch({executablePath:process.env.CHROME_PATH||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true});
    const context = await browser.newContext({viewport:{width:1440,height:900},timezoneId:'Asia/Shanghai',reducedMotion:'reduce'});
    const page = await context.newPage();
    page.setDefaultTimeout(6000);
    let targets = [northOne,account,west,northTwo], settingsFail = false, targetReads = 0;
    const runs = [], receipts = [], posted = [], captures = [], errors = [], unexpected = [], hashes = [];
    const finish = run => {
      run.status = 'completed';
      run.samples = [{model:run.config.models[0],index:0,execution_status:'completed',quality_verdict:'unknown',output:'21',answer_match:'correct'}];
    };
    page.on('pageerror',e=>errors.push(e.message));
    page.on('response',r=>{
      const u = new URL(r.url()), name = path.basename(u.pathname);
      if(u.origin===url.origin && assets.includes(name)) hashes.push(r.body().then(buf=>[name,createHash('sha256').update(buf).digest('hex')]));
    });
    const prefix = '/v0/management/plugins/model-quality';
    await context.route('**/*',async route=>{
      const r = route.request(), u = new URL(r.url()), api = u.pathname.slice(prefix.length);
      if(u.origin!==url.origin || r.method()!=='GET' && !(api==='/runs' && r.method()==='POST')){
        unexpected.push(r.method()+' '+u.pathname);return route.abort();
      }
      if(!u.pathname.startsWith(prefix+'/')) return route.continue();
      assert.equal(r.headers().authorization,'Bearer '+info.management_key);
      if(api==='/targets'){targetReads++;return route.fulfill({json:{items:targets,offset:0,total:targets.length}});}
      if(api==='/settings'){
        if(settingsFail)return route.fulfill({status:503,json:{error:{message:'模拟设置读取失败'}}});
        return route.fulfill({json:{include_provider_targets:false,capabilities:{mode:'official_observation',management_session:true}}});
      }
      if(api==='/runs' && r.method()==='POST'){
        const config = r.postDataJSON(), target = targets.find(t=>t.auth_id===config.auth_id);
        posted.push(config);
        const run = {id:'synthetic-iq-'+posted.length,source:'manual_test',status:'running',created_at:'2026-11-21T03:00:00Z',
          credential_email:target?.email,config:{...config,question,subject_auth_index:target?.auth_index,provider_name:target?.provider_name},samples:[]};
        runs.unshift(run);
        return route.fulfill({status:202,json:{run_id:run.id}});
      }
      if(api==='/runs/detail'){
        const key = u.searchParams.get('idempotency_key');
        if(key) receipts.push(key);
        const run = runs.find(e=>key?e.config.idempotency_key===key:e.id===u.searchParams.get('id'));
        return run?route.fulfill({json:run}):route.fulfill({status:404,json:{error:{code:'not_found'}}});
      }
      if(api==='/runs' && u.searchParams.get('source')==='manual_test'){
        return route.fulfill({json:{total:runs.length,offset:0,items:runs.map(row=>({...row,samples:row.samples.map(({model,index,execution_status,quality_verdict,answer_match})=>({model,index,execution_status,quality_verdict,answer_match}))}))}});
      }
      return route.continue();
    });
    url.searchParams.set('page','manual');url.searchParams.set('theme','light');url.searchParams.set('demo','1');
    await page.goto(url.href);
    const ui = page.frameLocator('#fixture-plugin'), el = selector=>ui.locator(selector);
    const frame = ()=>page.frames().find(f=>f.url().includes('/index.html'));
    const wait = (fn,arg)=>frame().waitForFunction(fn,arg,{timeout:6000});
    const modes = {provider:'#manual-kind-provider',credential:'#manual-kind-credential'};
    const mode = async kind=>{
      const input = el(modes[kind]);
      if(!await input.isChecked()){
        const label = await input.evaluate(node=>node.labels[0]?.getAttribute('for'));
        if(label) await el('label[for="'+label+'"]').click();
        else await el('label:has('+modes[kind]+')').click();
      }
      assert(await input.isChecked());
    };
    const choices = selector=>el(selector+' option').evaluateAll(opts=>opts.filter(o=>o.value&&!o.disabled).map(o=>o.value));
    const group = async name=>{
      const options = el('#manual-provider option');
      const matches = await options.evaluateAll((opts,title)=>opts.filter(o=>o.value&&o.textContent.includes(title)).map(o=>o.value),name);
      assert.equal(matches.length,1,'expected a unique group '+name);
      await el('#manual-provider').selectOption(matches[0]);
    };
    const ready = ()=>el('#run-submit:enabled').waitFor();
    const sameRequest = (body,target,model)=>{
      assert.equal(body.auth_id,target.auth_id);assert.equal(body.provider,target.provider);
      assert.deepEqual(body.models,[model]);assert.equal(body.question_id,question.id);
      assert.equal(body.judge.mode,'manual');assert(body.idempotency_key);
      for(const field of ['include_provider_targets','upstream_error_actions','managed_probe','provider_observation','subject_auth_index','rule_id'])
        assert(!Object.hasOwn(body,field),'manual submission included a rule or evidence field '+field);
    };
    const submit = async (target,model,selector='#run-submit')=>{
      const before=posted.length;
      const request=page.waitForRequest(r=>r.method()==='POST'&&new URL(r.url()).pathname===prefix+'/runs');
      await el(selector).click();
      sameRequest((await request).postDataJSON(),target,model);
      await el('#run-submit[aria-busy=true]').waitFor({state:'attached'});
      assert.equal(posted.length,before+1);
      return runs[0];
    };
    const refresh = async ()=>{
      const prev=targetReads;
      await el('#refresh').click();
      await el('#refresh:not([aria-busy=true])').waitFor({state:'attached'});
      assert(targetReads>prev,'refresh must read the current directory');
    };
    const snap = async (theme,width)=>{
      await page.locator('#fixture-theme').selectOption(theme);
      await wait(t=>document.documentElement.dataset.themeSource==='host' && document.documentElement.dataset.theme===t,theme);
      const dims = await el('#manual-setup').evaluate(n=>{
        const bounds=n.getBoundingClientRect();
        const overflow=['.manual-target-kind','#manual-provider','#target','#models'].filter(s=>{
          const control=n.querySelector(s);if(!control?.checkVisibility())return false;
          const rect=control.getBoundingClientRect();return rect.left<bounds.left-1 || rect.right>bounds.right+1;
        });
        return {width:n.clientWidth,scrollWidth:n.scrollWidth,root:document.documentElement.clientWidth,scrollRoot:document.documentElement.scrollWidth,overflow};
      });
      assert(dims.scrollWidth<=dims.width+1 && dims.scrollRoot<=dims.root+1,JSON.stringify(dims));
      assert.deepEqual(dims.overflow,[]);
      assert(await el('#manual-provider').isVisible());assert(await el('#models').isVisible());
      assert(await el('label:has(#manual-kind-provider)').isVisible());
      assert(await el('label:has(#manual-kind-credential)').isVisible());
      if(out)await el('#manual-setup').screenshot({path:path.join(out,theme+'-'+width+'-manual-provider.png'),animations:'disabled'});
      captures.push({theme,width,...dims});
    };
    if(out)await fs.mkdir(out,{recursive:true});
    await el('body.connected').waitFor();await el('[data-page=manual]').click();
    await el('#target:enabled').waitFor();assert(await el(modes.credential).isChecked());
    await el(modes.credential).focus();await page.keyboard.press('ArrowRight');
    assert(await el(modes.provider).isChecked());await page.keyboard.press('ArrowLeft');assert(await el(modes.credential).isChecked());
    assert.deepEqual(await choices('#target'),[account.auth_id]);
    await el('#target').selectOption(account.auth_id);await el('#models').selectOption('regular-only');await el('#question').selectOption(question.id);await ready();
    await el('[data-page=operations]').click();assert.equal(await el('#ops-include-providers').count(),0);await el('[data-page=manual]').click();
    await mode('provider');await group(northOne.provider_name);
    assert.deepEqual((await choices('#target')).sort(),[northOne.auth_id,northTwo.auth_id].sort());
    assert.equal(await el('#target').inputValue(),'');assert(await el('#run-submit').isDisabled());
    await el('#target').selectOption(northTwo.auth_id);
    assert.deepEqual((await choices('#models')).sort(),[...northTwo.models].sort());
    await el('#models').selectOption(shared);await ready();
    const first=await submit(northTwo,shared);
    // Changing the form during a running test prepares the next request independently of the frozen run.
    await group(west.provider_name);assert.equal(await el('#target').inputValue(),west.auth_id);
    await el('#models').selectOption(shared);assert(await el('#run-submit').isDisabled());assert.equal(posted.length,1);
    const originalReceipt=first.config.idempotency_key;
    await page.reload();await el('body.connected').waitFor();await el('[data-page=manual]').click();
    await el('#run-submit[aria-busy=true]').waitFor({state:'attached'});
    await wait(()=>document.querySelector('#manual-config-summary').textContent.includes('测试中'));
    assert(receipts.includes(originalReceipt),'reload must reconcile the original idempotency key');
    assert.equal(posted.length,1);
    await mode('provider');await group(west.provider_name);await el('#models').selectOption(shared);
    finish(first);await ready();
    const rerun=await submit(northTwo,shared,'#run-rerun');
    assert.notEqual(rerun.config.idempotency_key,originalReceipt);
    finish(rerun);await ready();
    const second=await submit(west,shared);finish(second);await ready();
    await mode('credential');await el('#target').selectOption(account.auth_id);await el('#models').selectOption('regular-only');
    const third=await submit(account,'regular-only');finish(third);await ready();
    await mode('provider');await group(northOne.provider_name);await el('#target').selectOption(northTwo.auth_id);await el('#models').selectOption(shared);
    await mode('credential');assert.equal(await el('#target').inputValue(),account.auth_id);assert.equal(await el('#models').inputValue(),'regular-only');
    await mode('provider');assert.equal(await el('#target').inputValue(),northTwo.auth_id);assert.equal(await el('#models').inputValue(),shared);
    // A failed operations settings read must have no effect on manual supplier admission.
    settingsFail=true;targets=[west,northTwo,account,northOne];await refresh();await ready();
    assert.equal(await el('#target').inputValue(),northTwo.auth_id);assert.equal(await el('#models').inputValue(),shared);
    targets=targets.filter(t=>t.auth_id!==northTwo.auth_id);await refresh();
    assert(await el('#run-submit').isDisabled());assert.notEqual(await el('#target').inputValue(),northOne.auth_id);
    await el('[data-manual-view=history]').click();
    const historical=el('#history-list [data-run="'+first.id+'"]');
    await historical.waitFor();assert((await historical.innerText()).includes(northTwo.provider_name));assert((await historical.innerText()).includes(northTwo.auth_index));
    await historical.click();
    const origin=el('#run-meta .answer-run-supplier');
    await origin.waitFor();assert((await origin.innerText()).includes(northTwo.provider_name));assert((await origin.innerText()).includes(northTwo.auth_index));
    assert(await el('#run-rerun').isDisabled());
    targets=[northOne,account,{...northTwo,auth_index:'replacement-index'},west];await refresh();
    assert(await el('#run-rerun').isDisabled());assert((await origin.innerText()).includes(northTwo.auth_index));
    targets=[northOne,account,{...northTwo,models:['north-second']},west];await refresh();
    assert(await el('#run-submit').isDisabled());assert.notEqual(await el('#models').inputValue(),'north-second');
    const priorPause = { ...northTwo, credential_handling: 'disabled',
      error_policy_block: { code: 'error_policy_disabled', reason: 'CPA 注册目录缺少所选模型', model: 'former-model' } };
    const showPolicyRoute = async current => {
      targets = [northOne, account, current, west];
      await refresh();
    };
    await showPolicyRoute(priorPause);
    await ready();
    assert.equal(await el('#target-summary').textContent(), '通过 CPA 原生路由测试');
    const recovered = await submit(priorPause, shared);
    finish(recovered);
    await ready();
    await showPolicyRoute({ ...priorPause, runtime_status: 'active' });
    await ready();
    const unavailableRoutes = [
      [{ runtime_status: 'disabled', manual_disabled: true, effective_available: false }, /CPA 当前仍停用此供应商路由/],
      [{ effective_available: false }, /原生路由尚未确认/],
      [{ runtime_status: 'active', effective_available: false, reasons: ['native_cooldown'] }, /正在 CPA 冷却/],
      [{ runtime_status: 'error', effective_available: false, reasons: ['native_unavailable'] }, /CPA 标记为不可用/],
      [{ runtime_status: 'unknown', effective_available: false, reasons: ['provider_route_unverified'] }, /原生路由尚未确认/],
      [{ auth_index: '' }, /原生路由尚未确认/],
      [{ effective_available: false, reasons: ['identity_conflict'] }, /路由标识不一致/],
      [{ models: [], reasons: ['provider_models_unavailable'] }, /暂无已注册模型/],
      [{ models: ['north-second'] }, /没有所选模型 shared-model/],
      [{ retained_pause: true }, /质量暂停/],
    ];
    for (const [patch, message] of unavailableRoutes) {
      await showPolicyRoute({ ...priorPause, ...patch });
      assert(await el('#run-submit').isDisabled());
      assert.match(await el('#target-summary').textContent(), message);
      assert.equal(posted.length, 5);
    }
    targets=[northOne,account,northTwo,{...west,effective_available:false,runtime_status:'unknown',reasons:['provider_route_unverified']}];await refresh();
    await group(west.provider_name);assert(await el('#run-submit').isDisabled());
    targets=[account];await refresh();assert(await el('#run-submit').isDisabled());assert.equal(posted.length,5);
    const namesake = provider('secondary-north','north-01','other-executor',northOne.provider_name,[shared]);
    const otherGroup = provider('north-lab','lab-01',northOne.provider,'北辰试验组',[shared]);
    targets=[namesake,account,northOne,northTwo,west,otherGroup];await refresh();
    const matchingGroups=await el('#manual-provider option').evaluateAll((opts,name)=>opts.filter(o=>o.value && o.textContent.includes(name)).map(o=>o.value),northOne.provider_name);
    assert.equal(matchingGroups.length,2);
    const lists=[];
    for(const key of matchingGroups){await el('#manual-provider').selectOption(key);lists.push((await choices('#target')).sort().join(','));}
    assert.deepEqual(lists.sort(),[namesake.auth_id,[northOne.auth_id,northTwo.auth_id].sort().join(',')].sort());
    await group(otherGroup.provider_name);assert.equal(await el('#target').inputValue(),otherGroup.auth_id);
    settingsFail=false;targets=[account,northOne,northTwo,west];await refresh();
    await group(northOne.provider_name);await el('#target').selectOption(northTwo.auth_id);await el('#models').selectOption(shared);await ready();
    for(const [width,height] of [[1440,900],[390,844],[320,750]]){
      await page.setViewportSize({width,height});
      if(width<900)await el('[data-manual-pane=setup]').click();
      for(const theme of ['light','white','dark']){
        await page.locator('#fixture-theme').selectOption(theme);
        await wait(t=>document.documentElement.dataset.theme===t,theme);
        await mode('credential');assert.equal(await el('#target').inputValue(),account.auth_id);assert.equal(await el('#models').inputValue(),'regular-only');
        await mode('provider');await group(west.provider_name);assert.equal(await el('#target').inputValue(),west.auth_id);
        await group(northOne.provider_name);await el('#target').selectOption(northOne.auth_id);await el('#models').selectOption(shared);
        await el('#target').selectOption(northTwo.auth_id);await el('#models').selectOption(shared);await ready();
        await el('#manual-setup').evaluate(card=>{card.scrollTop=0;});
        await snap(theme,width);
      }
    }
    assert.deepEqual(errors,[]);assert.deepEqual(unexpected,[]);assert.equal(posted.length,5);
    const ui_source_sha256=Object.fromEntries(await Promise.all(hashes));assert.deepEqual(Object.keys(ui_source_sha256).sort(),assets.sort());
    const version=JSON.parse(await el('#build-info').textContent()).ui_version;
    if(out)await fs.writeFile(path.join(out,'manual-supplier-report.json'),JSON.stringify({version,synthetic:true,posted:posted.map(({auth_id,provider,models})=>({auth_id,provider,models})),verifiedReceipt:true,captures,ui_source_sha256,errors,unexpected},null,2));
    console.log('Verified five isolated manual submissions, provider admission after a historical suspension, frozen identity on rerun/reload, remembered model choices, directory changes and nine theme/size combinations.');
  } finally {await browser?.close();fixture.kill();await ended;}
}
test().catch(e=>{console.error(e);process.exitCode=1;});
