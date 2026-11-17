import test from 'node:test';
import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const root=fileURLToPath(new URL('..',import.meta.url));

async function worker(value,args=[]) {
  return new Promise((resolve,reject)=>{
    const process=spawn(globalThis.process.execPath,[root+'/worker.mjs',...args],{stdio:['pipe','pipe','pipe'],env:{PATH:'/usr/bin:/bin',HOME:globalThis.process.env.HOME}});
    let stdout='',stderr='';
    process.stdout.on('data',chunk=>stdout+=chunk);
    process.stderr.on('data',chunk=>stderr+=chunk);
    process.on('error',reject);
    process.on('close',code=>resolve({code,stdout,stderr}));
    process.stdin.on('error',()=>{});
    process.stdin.end(value);
  });
}

test('worker rejects oversized, malformed and unsupported IPC without echoing secrets',async()=>{
  const secret='private-password-do-not-print';
  for(const payload of [secret,JSON.stringify({version:1,password:secret}),JSON.stringify({version:1,padding:secret.repeat(1000)})]) {
    const result=await worker(payload);
    assert.equal(result.code,1);
    assert.equal(JSON.parse(result.stdout).error_code,'invalid_input');
    assert.equal(result.stderr,'');
    assert(!result.stdout.includes(secret));
  }
});

test('worker accepts no secret CLI arguments',async()=>{
  const result=await worker('', ['private-password-do-not-print']);
  assert.equal(JSON.parse(result.stdout).error_code,'invalid_input');
  assert.equal(result.stderr,'');
  assert(!result.stdout.includes('private-password'));
});

test('worker reports only fixed proxy diagnostics without sending credentials to the browser',async()=>{
  const redirect='http://localhost:1455/auth/callback';
  const value={version:1,job_id:'offline-proxy-job',attempt:1,state:'offline-state',redirect_uri:redirect,credentials:{email:'fixture@example.test',password:'never-echo-password',totp_secret:'GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ'},proxy_url:'socks5h://proxy-user:never-echo-proxy@127.0.0.1:1080',authorization_url:'https://auth.openai.com/oauth/authorize?'+new URLSearchParams({client_id:'app_EMoamEEZ73f0CkXaXp7hrann',redirect_uri:redirect,state:'offline-state',response_type:'code',code_challenge:'a'.repeat(43),code_challenge_method:'S256',scope:'openid profile email offline_access'})};
  const result=await worker(JSON.stringify(value));
  assert.deepEqual(JSON.parse(result.stdout),{version:1,job_id:'offline-proxy-job',attempt:1,error_code:'proxy_unsupported',error_stage:'browser_start'});
  assert.equal(result.code,1);
  assert.equal(result.stderr,'');
  for(const secret of ['never-echo-password','never-echo-proxy','fixture@example.test',value.credentials.totp_secret])assert(!result.stdout.includes(secret));
});

test('browser subprocess inherits the host worker process group',async()=>{
  const script=`import { retainProcessGroup } from './process-scope.mjs';retainProcessGroup(); const { chromium } = await import('playwright-core'); const {browserExecutable}=await import('./core.mjs'); const browser=await chromium.launch({executablePath:browserExecutable(),headless:true}); console.log('READY'); await new Promise(resolve=>setTimeout(resolve,30000)); await browser.close();`;
  const child=spawn(process.execPath,['--input-type=module','-e',script],{cwd:root,detached:true,stdio:['ignore','pipe','pipe'],env:{PATH:'/usr/bin:/bin',HOME:process.env.HOME}});
  let ready='';
  try {
    await new Promise((resolve,reject)=>{child.stdout.on('data',chunk=>{ready+=chunk;if(ready.includes('READY'))resolve();});child.once('exit',()=>reject(new Error('browser failed')));});
    const rows=execFileSync('/bin/ps',['-axo','pid=,ppid=,pgid='],{encoding:'utf8'}).trim().split('\n').map(line=>line.trim().split(/\s+/).map(Number));
    const descendants=new Set([child.pid]);let expanded=true;
    while(expanded){expanded=false;for(const [pid,parent] of rows)if(descendants.has(parent)&&!descendants.has(pid)){descendants.add(pid);expanded=true;}}
    assert(descendants.size>1);
    for(const [pid,,group] of rows)if(descendants.has(pid))assert.equal(group,child.pid);
  } finally {
    process.kill(-child.pid,'SIGKILL');
    await new Promise(resolve=>child.once('close',resolve));
  }
});
