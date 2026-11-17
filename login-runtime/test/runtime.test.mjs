import test from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import { once } from 'node:events';
import { chromium } from 'playwright-core';
import { browserExecutable, browserProxy, errorDiagnostics, LoginError, runLogin, validateInput, validateCallback } from '../core.mjs';
import { generateTotp, normalizeTotpSecret } from '../totp.mjs';

const secret = 'GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ';
const identity = { version: 1, job_id: 'offline-job', attempt: 1, state: 'fixture-state', redirect_uri: 'http://localhost:1455/auth/callback', credentials: { email: 'fixture@example.test', password: 'only-for-offline-tests', totp_secret: secret } };
const input = origin => ({ ...identity, authorization_url: `${origin}/oauth/authorize?${new URLSearchParams({client_id:'app_EMoamEEZ73f0CkXaXp7hrann',redirect_uri:identity.redirect_uri,state:identity.state,response_type:'code',code_challenge:'a'.repeat(43),code_challenge_method:'S256',scope:'openid profile email offline_access'})}` });

test('RFC 6238 SHA1 vectors use six digits and explicit clock', () => {
  for (const [timestamp, expected] of [[59, '287082'],[1111111109,'081804'],[1111111111,'050471'],[1234567890,'005924'],[2000000000,'279037'],[20000000000,'353130']]) assert.equal(generateTotp(secret,timestamp*1000),expected);
  assert.equal(generateTotp(secret,29999),generateTotp(secret,0));
  assert.notEqual(generateTotp(secret,30000),generateTotp(secret,29999));
  assert.equal(normalizeTotpSecret(`otpauth://totp/test?secret=${secret}&algorithm=SHA1&digits=6&period=30`),secret);
  for (const value of ['123456','INVALID!','otpauth://hotp/test?secret='+secret,'otpauth://totp/test?secret='+secret+'&digits=8']) assert.throws(()=>normalizeTotpSecret(value));
});

test('production request rejects arbitrary endpoints, PKCE changes, unknown fields and callback replay parameters', () => {
  const value = input('https://auth.openai.com');
  assert.equal(validateInput(value),value);
  assert.throws(()=>validateInput(input('http://127.0.0.1:1234')));
  assert.throws(()=>validateInput({...value, verify_after_login:true}));
  assert.throws(()=>validateInput({...value,browserVerificationTimeoutMs:300}));
  assert.throws(()=>validateInput({...value,policy:{browserVerificationOrigin:'https://other.example'}}));
  assert.throws(()=>validateInput({...value,state:'wrong'}));
  assert.throws(()=>validateInput({...value,redirect_uri:'https://evil.example/auth/callback'}));
  const callback = `${value.redirect_uri}?code=once&state=${value.state}`;
  assert.equal(validateCallback(callback,value),callback);
  for(const invalid of [callback+'&code=two',callback+'&state=two',callback.replace('localhost','127.0.0.1'),callback.replace('fixture-state','wrong')]) assert.throws(()=>validateCallback(invalid,value));
});

async function fakeFlow(t, scenario='success', verificationOrigin='') {
  const calls = [];
  const challengePath = '/cdn-cgi/challenge-platform/h/g/flow/ov1/offline-fixture';
  const server = http.createServer(async(req,res)=>{
    const route = new URL(req.url,'http://localhost').pathname;
    let body='';for await(const chunk of req)body+=chunk;
    calls.push({method:req.method,path:route,body});
    const html=(body)=>{res.writeHead(200,{'content-type':'text/html'});res.end(`<!doctype html><html><body>${body}</body></html>`);};
    const form=(label,type,name,action)=>`<h1>${label}</h1><form method="post" action="${action}"><input type="${type}" name="${name}" autocomplete="${name==='code'?'one-time-code':name}"><button>Continue</button></form>`;
    if(route==='/oauth/authorize') {
      if(scenario.startsWith('challenge_')) {
        const pending = '<h1>Just a moment...</h1><p>Checking your browser before continuing.</p>';
        if(scenario==='challenge_external') return html(pending+`<script src="${verificationOrigin}/turnstile/v0/api.js"></script>`);
        if(scenario==='challenge_external_navigation') return html(pending+`<script>location.assign('${verificationOrigin}/turnstile/v0/api.js');</script>`);
        if(scenario==='challenge_external_unknown_write') return html(pending+`<script>fetch('${verificationOrigin}/turnstile/not-a-challenge-write',{method:'POST',mode:'no-cors',body:'offline-check'}).catch(()=>{}).finally(()=>location.assign('/log-in-or-create-account'));</script>`);
        if(scenario==='challenge_stuck') return html(pending+form('Email','email','email','/api/accounts/authorize/continue')+`<script>fetch('${challengePath}',{method:'POST',body:'offline-check'}).catch(()=>{});</script>`);
        if(scenario==='challenge_navigation') return html(pending+`<form method="post" action="${challengePath}"><input name="check" value="offline"></form><script>document.forms[0].submit();</script>`);
        if(scenario==='challenge_put') return html(pending+`<script>fetch('${challengePath}',{method:'PUT',body:'offline-check'}).catch(()=>{});</script>`);
        const next = "location.assign('/log-in-or-create-account')";
        if(scenario==='challenge_xhr') return html(pending+`<script>const xhr=new XMLHttpRequest();xhr.open('POST','${challengePath}');xhr.onload=()=>{${next}};xhr.send('offline-check');</script>`);
        return html(pending+`<script>fetch('${challengePath}',{method:'POST',body:'offline-check'}).then(()=>{${next}}).catch(()=>{});</script>`);
      }
      if(scenario==='login_entry' || scenario==='forbidden_registration') {
        res.writeHead(302,{location:scenario==='login_entry'?'/log-in-or-create-account':'/create-account'});res.end();return;
      }
      return html(form('Email','email','email',scenario==='forbidden_registration_post'?'/api/accounts/create_account':scenario==='unsupported_request'?'/api/accounts/unknown-login-action':'/api/accounts/authorize/continue'));
    }
    if(route===challengePath) {
      if(scenario==='challenge_fetch'||scenario==='challenge_xhr') await new Promise(resolve=>setTimeout(resolve,400));
      res.writeHead(204);res.end();return;
    }
    if(route==='/log-in-or-create-account')return html(form('Email','email','email','/api/accounts/authorize/continue'));
    if(route==='/api/accounts/authorize/continue')return html(form('Password','password','password','/api/accounts/password/verify'));
    if(route==='/api/accounts/password/verify') {
      if(scenario==='password_rejected'){res.writeHead(401);res.end('Incorrect password');return;}
      if(scenario==='interaction_required')return html('<h1>Verify you are human</h1>');
      if(scenario==='totp_alternatives')return html(form('Authenticator app MFA','text','code','/api/accounts/mfa/verify')+'<a href="/email-verification">Try email code instead</a><button>Try SMS instead</button>');
      if(scenario==='email_code')return html(form('MFA: Enter the email code sent to your inbox','text','code','/api/accounts/mfa/verify')+'<button>Authenticator app</button>');
      return html(form('Authenticator app MFA','text','code','/api/accounts/mfa/verify'));
    }
    if(route==='/api/accounts/mfa/verify') {
      if(scenario==='totp_rejected'){res.writeHead(401);res.end('Invalid code');return;}
      assert.equal(new URLSearchParams(body).get('code'),generateTotp(secret,59000));
      res.writeHead(302,{location:`${identity.redirect_uri}?code=offline-code&state=${scenario==='state_mismatch'?'incorrect':identity.state}`});res.end();return;
    }
    res.writeHead(404);res.end();
  });
  server.listen(0,'127.0.0.1');await once(server,'listening');
  t.after(()=>new Promise(resolve=>{server.closeAllConnections();server.close(resolve);}));
  const origin=`http://127.0.0.1:${server.address().port}`;
  return {value:input(origin),calls,options:{chromium,executablePath:browserExecutable(),policy:{directOAuthFixture:true,authOrigin:origin},now:()=>59000}};
}

async function fakeVerification(t) {
  const calls=[];
  let origin;
  const server=http.createServer(async(req,res)=>{
    const route=new URL(req.url,'http://localhost').pathname;
    let body='';for await(const chunk of req)body+=chunk;
    calls.push({method:req.method,path:route,body});
    if(route==='/turnstile/v0/api.js') {
      res.writeHead(200,{'content-type':'text/javascript'});
      res.end(`window.addEventListener('message',event=>{if(event.origin==='${origin}'&&event.data==='offline-complete')location.assign('/log-in-or-create-account');});const frame=document.createElement('iframe');frame.src='${origin}/cdn-cgi/challenge-platform/offline-frame';document.body.append(frame);`);
      return;
    }
    if(route==='/cdn-cgi/challenge-platform/offline-frame') {
      res.writeHead(200,{'content-type':'text/html'});
      res.end(`<!doctype html><script>fetch('/cdn-cgi/challenge-platform/offline-check',{method:'POST',body:'offline-check'}).then(()=>parent.postMessage('offline-complete','*'));</script>`);
      return;
    }
    if(route==='/cdn-cgi/challenge-platform/offline-check') {
      await new Promise(resolve=>setTimeout(resolve,400));
      res.writeHead(204);res.end();return;
    }
    res.writeHead(404);res.end();
  });
  server.listen(0,'127.0.0.1');await once(server,'listening');
  t.after(()=>new Promise(resolve=>{server.closeAllConnections();server.close(resolve);}));
  origin=`http://127.0.0.1:${server.address().port}`;
  return {origin,calls};
}

test('real Chromium completes one password/TOTP attempt and returns host callback without sending callback or token requests', async t=>{
  const {value,calls,options}=await fakeFlow(t);
  const result=await runLogin(value,options);
  assert.equal(result.callback_url,`${identity.redirect_uri}?code=offline-code&state=${identity.state}`);
  assert.equal(calls.filter(c=>c.path==='/api/accounts/password/verify').length,1);
  assert.equal(calls.filter(c=>c.path==='/api/accounts/mfa/verify').length,1);
  assert(!calls.some(c=>c.path.includes('token')||c.path.includes('callback')));
});

for(const scenario of ['login_entry','totp_alternatives']) {
  test(`normal login completes for ${scenario} without false interaction-required classification`,async t=>{
    const {value,calls,options}=await fakeFlow(t,scenario);
    const result=await runLogin(value,options);
    assert.equal(result.callback_url,`${identity.redirect_uri}?code=offline-code&state=${identity.state}`);
    assert.equal(calls.filter(c=>c.path==='/api/accounts/password/verify').length,1);
    assert.equal(calls.filter(c=>c.path==='/api/accounts/mfa/verify').length,1);
    assert(!calls.some(c=>c.path==='/create-account'||c.path==='/api/accounts/create_account'||c.path==='/email-verification'));
  });
}

for(const scenario of ['challenge_fetch','challenge_xhr']) {
  test(`official automatic browser verification completes through ${scenario} before submitting credentials`,async t=>{
    const {value,calls,options}=await fakeFlow(t,scenario);
    const result=await runLogin(value,{...options,policy:{...options.policy,browserVerificationTimeoutMs:2000}});
    assert.equal(result.callback_url,`${identity.redirect_uri}?code=offline-code&state=${identity.state}`);
    const challenge=calls.filter(call=>call.path.startsWith('/cdn-cgi/challenge-platform/'));
    assert.equal(challenge.length,1);
    assert.equal(challenge[0].method,'POST');
    const next=calls.findIndex(call=>call.path==='/log-in-or-create-account');
    const firstCredential=calls.findIndex(call=>call.path==='/api/accounts/authorize/continue');
    assert(next>calls.indexOf(challenge[0]));
    assert(firstCredential>next);
    for(const endpoint of ['/api/accounts/authorize/continue','/api/accounts/password/verify','/api/accounts/mfa/verify']) assert.equal(calls.filter(call=>call.path===endpoint).length,1);
    assert(!calls.some(call=>call.path.includes('token')||call.path.includes('callback')));
  });
}

test('unfinished official browser verification stops with human verification and never submits a visible login form',async t=>{
  const {value,calls,options}=await fakeFlow(t,'challenge_stuck');
  await assert.rejects(runLogin(value,{...options,policy:{...options.policy,browserVerificationTimeoutMs:300}}),{code:'interaction_required',stage:'authorization',reason:'human_verification'});
  assert.equal(calls.filter(call=>call.method==='POST'&&call.path.startsWith('/cdn-cgi/challenge-platform/')).length,1);
  assert(!calls.some(call=>call.path.startsWith('/api/accounts/')));
});

for(const scenario of ['challenge_navigation','challenge_put']) {
  test(`browser verification permission rejects ${scenario} without sending its request`,async t=>{
    const {value,calls,options}=await fakeFlow(t,scenario);
    await assert.rejects(runLogin(value,{...options,policy:{...options.policy,browserVerificationTimeoutMs:300}}),{code:'interaction_required',stage:'authorization',reason:'unsupported_request'});
    assert(!calls.some(call=>call.path.startsWith('/cdn-cgi/challenge-platform/')||call.path.startsWith('/api/accounts/')));
  });
}

test('official verification origin can load its script and child frame and finish its own check',async t=>{
  const verification=await fakeVerification(t);
  const {value,calls,options}=await fakeFlow(t,'challenge_external',verification.origin);
  const result=await runLogin(value,{...options,policy:{...options.policy,browserVerificationOrigin:verification.origin,browserVerificationTimeoutMs:2000}});
  assert.equal(result.callback_url,`${identity.redirect_uri}?code=offline-code&state=${identity.state}`);
  assert.deepEqual(verification.calls.map(({method,path})=>({method,path})),[
    {method:'GET',path:'/turnstile/v0/api.js'},
    {method:'GET',path:'/cdn-cgi/challenge-platform/offline-frame'},
    {method:'POST',path:'/cdn-cgi/challenge-platform/offline-check'},
  ]);
  assert.equal(calls.filter(call=>call.path==='/api/accounts/password/verify').length,1);
  assert.equal(calls.filter(call=>call.path==='/api/accounts/mfa/verify').length,1);
});

test('verification origin cannot replace the main authorization page',async t=>{
  const verification=await fakeVerification(t);
  const {value,calls,options}=await fakeFlow(t,'challenge_external_navigation',verification.origin);
  await assert.rejects(runLogin(value,{...options,policy:{...options.policy,browserVerificationOrigin:verification.origin,browserVerificationTimeoutMs:300}}),{code:'interaction_required',stage:'authorization',reason:'unsupported_navigation'});
  assert.deepEqual(verification.calls,[]);
  assert(!calls.some(call=>call.path.startsWith('/api/accounts/')));
});

test('verification resource permission does not send unrelated writes to that origin',async t=>{
  const verification=await fakeVerification(t);
  const {value,options}=await fakeFlow(t,'challenge_external_unknown_write',verification.origin);
  const result=await runLogin(value,{...options,policy:{...options.policy,browserVerificationOrigin:verification.origin,browserVerificationTimeoutMs:2000}});
  assert.equal(result.callback_url,`${identity.redirect_uri}?code=offline-code&state=${identity.state}`);
  assert.deepEqual(verification.calls,[]);
});

for(const [scenario,stage,reason] of [
  ['forbidden_registration','authorization','unsupported_navigation'],
  ['forbidden_registration_post','email','unsupported_navigation'],
  ['unsupported_request','email','unsupported_request'],
  ['interaction_required','password','human_verification'],
  ['email_code','totp','totp_method_unconfirmed'],
]) {
  test(`fixed diagnostic identifies ${scenario} without recording page content`,async t=>{
    const {value,calls,options}=await fakeFlow(t,scenario);
    await assert.rejects(runLogin(value,options),error=>{
      assert.equal(error.code,'interaction_required');
      assert.deepEqual(errorDiagnostics(error),{error_stage:stage,error_reason:reason});
      assert.equal(error.message,'interaction_required');
      return true;
    });
    assert(!calls.some(c=>c.method==='POST'&&(c.path==='/create-account'||c.path==='/api/accounts/create_account'||c.path==='/api/accounts/unknown-login-action')));
    assert.equal(calls.filter(c=>c.path==='/api/accounts/mfa/verify').length,0);
  });
}

test('proxy settings preserve HTTP credentials, normalize remote SOCKS DNS and reject unsupported authentication',()=>{
  assert.deepEqual(browserProxy('http://proxy-user:p%40ss@127.0.0.1:8080'),{server:'http://127.0.0.1:8080',username:'proxy-user',password:'p@ss'});
  assert.deepEqual(browserProxy('socks5h://127.0.0.1:1080'),{server:'socks5://127.0.0.1:1080'});
  for(const direct of ['',undefined,' direct ','off','none'])assert.equal(browserProxy(direct),undefined);
  for(const invalid of ['ftp://localhost:12','http://localhost:8080/path','http://localhost:8080?x=1','http://localhost:8080#x','http://localhost:8080?','http://localhost:8080#','http://localhost:0','http://:pw@localhost:8080','http://%zz:pw@localhost:8080','http://user:p%09w@localhost:8080',{}])assert.throws(()=>browserProxy(invalid),{code:'invalid_input'});
  assert.throws(()=>browserProxy('socks5h://user:password@localhost:1080'),{code:'proxy_unsupported',stage:'browser_start'});
  assert.deepEqual(errorDiagnostics({stage:identity.credentials.password,reason:identity.credentials.email}),{});
  assert.deepEqual(errorDiagnostics(new LoginError('interaction_required',identity.credentials.password,identity.credentials.email)),{});
});

test('direct login explicitly disables system proxies while configured login uses its exact proxy',async()=>{
  for(const value of ['', 'direct', 'http://127.0.0.1:8080']) {
    let launchOptions;
    const chromium={launch:async options=>{launchOptions=options;throw new Error('offline launch probe');}};
    await assert.rejects(runLogin({...input('https://auth.openai.com'),proxy_url:value},{chromium}),{code:'transport_error',stage:'browser_start'});
    assert.equal(launchOptions.args.includes('--no-proxy-server'),!value||value==='direct');
    assert.deepEqual(launchOptions.proxy,value.startsWith('http')?{server:value}:undefined);
  }
});

test('real Chromium routes the email, password and TOTP forms through the configured authenticated HTTP proxy',async t=>{
  const {calls,options}=await fakeFlow(t,'totp_rejected');
  const target=new URL(options.policy.authOrigin);
  const forwarded=[];
  const expected='Basic '+Buffer.from('offline-user:offline-password').toString('base64');
  const proxy=http.createServer((req,res)=>{
    if(req.headers['proxy-authorization']!==expected){res.writeHead(407,{'proxy-authenticate':'Basic realm="offline"'});res.end();return;}
    let url;try{url=new URL(req.url);}catch{res.writeHead(400);res.end();return;}
    if(url.hostname!=='offline-auth.test'){res.writeHead(502);res.end();return;}
    forwarded.push({method:req.method,path:url.pathname});
    const headers={...req.headers,host:target.host};delete headers['proxy-authorization'];
    const upstream=http.request({hostname:target.hostname,port:target.port,path:url.pathname+url.search,method:req.method,headers},reply=>{res.writeHead(reply.statusCode,reply.headers);reply.pipe(res);});
    upstream.on('error',()=>{if(!res.headersSent)res.writeHead(502);res.end();});req.pipe(upstream);
  });
  proxy.on('connect',(_req,socket)=>socket.end('HTTP/1.1 502 Bad Gateway\r\n\r\n'));
  proxy.listen(0,'127.0.0.1');await once(proxy,'listening');
  t.after(()=>new Promise(resolve=>{proxy.closeAllConnections();proxy.close(resolve);}));
  const value={...input('http://offline-auth.test'),proxy_url:`http://offline-user:offline-password@127.0.0.1:${proxy.address().port}`};
  await assert.rejects(runLogin(value,{...options,policy:{directOAuthFixture:true,authOrigin:'http://offline-auth.test'}}),{code:'totp_rejected',stage:'totp',reason:'upstream_rejected'});
  for(const endpoint of ['/oauth/authorize','/api/accounts/authorize/continue','/api/accounts/password/verify','/api/accounts/mfa/verify'])assert.equal(forwarded.filter(call=>call.path===endpoint).length,1);
  assert.equal(calls.filter(call=>call.path==='/api/accounts/mfa/verify').length,1);
});
for(const scenario of ['password_rejected','totp_rejected','interaction_required','state_mismatch']) {
  test(`single attempt stops on ${scenario}`,async t=>{
    const {value,calls,options}=await fakeFlow(t,scenario);
    await assert.rejects(runLogin(value,options),{code:scenario==='state_mismatch'?'callback_state_mismatch':scenario});
    assert.equal(calls.filter(c=>c.path==='/api/accounts/password/verify').length,1);
    assert(calls.filter(c=>c.path==='/api/accounts/mfa/verify').length<=1);
  });
}

test('cancelled acquisition closes Chromium and does not submit credentials',async t=>{
  const {value,calls,options}=await fakeFlow(t);
  const controller=new AbortController();controller.abort();
  await assert.rejects(runLogin(value,{...options,signal:controller.signal}),{code:'cancelled'});
  assert.equal(calls.filter(c=>c.method==='POST').length,0);
});

test('cancellation during a pending official page closes Chromium promptly',async t=>{
  let reached;
  const requested=new Promise(resolve=>{reached=resolve;});
  const server=http.createServer((_req,_res)=>reached());
  server.listen(0,'127.0.0.1');await once(server,'listening');
  t.after(()=>new Promise(resolve=>{server.closeAllConnections();server.close(resolve);}));
  const origin=`http://127.0.0.1:${server.address().port}`;
  const controller=new AbortController();
  const result=runLogin(input(origin),{chromium,policy:{directOAuthFixture:true,authOrigin:origin},signal:controller.signal});
  await requested;
  controller.abort();
  await assert.rejects(result,{code:'cancelled'});
});
