// Presentation helpers have no access to management credentials or task state.
export const escapeHTML = (value) =>
  String(value ?? '').replace(
    /[&<>"']/g,
    (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c],
  );
export const items = (data) => (Array.isArray(data) ? data : data?.items || []);
export const targetID = (target) => target.auth_id || target.id;
export const parseModels = (text) => [
  ...new Set(
    text
      .split(/[\n,]/)
      .map((part) => part.trim())
      .filter(Boolean),
  ),
];
export const active = (run) => ['queued', 'running', 'cancelling'].includes(run?.status);
export const date = (time) => {
  if (!time || String(time).startsWith('0001-')) return '未知';
  const parsed = new Date(time);
  return Number.isNaN(parsed.getTime()) ? '未知' : parsed.toLocaleString('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
  });
};
export const duration = (ms) => ms == null || !Number.isFinite(Number(ms)) || Number(ms) < 0
  ? '未知' : Number(ms) < 1000 ? `${Number(ms).toLocaleString('zh-CN', { maximumFractionDigits: 2 })} ms` : `${(Number(ms) / 1000).toFixed(2)} s`;
const sourceLabels = { manual_test: '智商测试', scheduled_probe: '规则检测', production: '业务请求', judge: '独立判题', internal_other: '其他内部请求', unknown: '来源未知' };
export const sourceLabel = (source) => sourceLabels[source] || source || '来源未知';
const reasonLabels = {
  cancelled: '测试已取消',
  'context canceled': '测试已取消',
  'exact answer': '答案与冻结参考答案一致',
  'integer differs from frozen reference': '整数答案与冻结参考答案不一致',
  'manual assessment required': '需要人工评审',
  'answer is not exactly one integer': '回答不符合仅输出一个整数的格式，无法判定',
  'invalid integer': '整数格式无效，无法判定',
  'refusal is not quality evidence': '模型拒答，不作为答题质量失败证据',
  'missing or truncated completion': '回答未完整结束或已截断，不作为答题质量失败证据',
  'judge budget exhausted': '独立判题预算不足，无法判定',
  'judge execution failed': '独立判题执行失败，无法判定',
  'invalid judge JSON': '独立判题未返回有效 JSON，无法判定',
  'judge must return an object': '独立判题未返回 JSON 对象，无法判定',
  'invalid key': '独立判题返回的字段名无效，无法判定',
  'duplicate or unexpected judge field': '独立判题返回重复或未约定字段，无法判定',
  'judge fields must be strings': '独立判题返回的字段类型无效，无法判定',
  'invalid judge object': '独立判题返回的对象不完整，无法判定',
  'extra judge output': '独立判题包含额外输出，无法判定',
  'invalid judge verdict': '独立判题结论格式无效，无法判定',
  'all selected models explicitly failed complete comparable rounds; manual release required': '所有选定模型在完整、可比较的连续轮次中明确答错，须人工解除暂停',
};
export const reasonLabel = (reason) => reasonLabels[reason] || reason || '未提供说明';
const verdictLabels = { correct: '正确', incorrect: '错误', unknown: '无法判断' };
export const statusLabels = {
  queued: '排队中',
  running: '执行中',
  completed: '已完成',
  complete: '已完成',
  success: '完成',
  succeeded: '完成',
  failed: '执行失败',
  error: '执行失败',
  cancelled: '已取消',
  canceling: '正在取消',
  cancelling: '正在取消',
  interrupted: '已中断',
  detecting: '检测中',
  awaiting_release: '等待人工解除',
  credential_paused: '凭证已暂停',
  blocked: '受阻',
  skipped: '跳过',
  pending: '等待中',
  observe_only: '仅观察',
  paused: '凭证已暂停',
  released: '已释放',
  conflict: '归属冲突',
  none: '无动作',
  not_applicable: '不适用',
  refused: '模型拒答',
  incomplete: '回答不完整',
  empty: '空回答',
};
export const tag = (text, tone = 'neutral') => `<span class="tag ${tone}">${escapeHTML(text)}</span>`;
export const statusTag = (status) =>
  tag(
    statusLabels[status] || status || '未知',
    ['failed', 'error', 'conflict', 'blocked'].includes(status)
      ? 'bad'
      : ['queued', 'running'].includes(status)
        ? 'warn'
        : 'neutral',
  );
export const verdictTag = (verdict) =>
  tag(
    verdictLabels[verdict] || '无法判断',
    verdict === 'correct' ? 'good' : verdict === 'incorrect' ? 'bad' : 'neutral',
  );

// Editing any part of a built-in prompt detaches its reference answer.
export function questionTestSelection(question, prompt) {
  const text = String(prompt ?? '');
  if (question?.id && question.id !== 'custom' && text === question.prompt) return { question_id: question.id };
  return { question_id: 'custom', prompt: text };
}

export function copyRunConfig(source) {
  const config = {};
  for (const key of [
    'auth_id',
    'provider',
    'models',
    'question_id',
    'prompt',
    'review_standard',
    'seed',
    'samples',
    'parameters',
    'judge',
    'control_auth_id',
    'control_provider',
  ]) {
    if (source[key] !== undefined) config[key] = source[key];
  }
  return structuredClone(config);
}

export function credentialBlock(config, targets) {
  const references = [
    ['目标', config.auth_id],
    ['对照', config.control_auth_id],
    ['判题', config.judge?.mode === 'llm' ? config.judge.auth_id : ''],
  ];
  for (const [role, id] of references) {
    if (!id) continue;
    const target = targets.find((entry) => targetID(entry) === id);
    if (!target) return `${role}凭证当前不可发现，请刷新并核对状态。`;
    if (target.quality_paused)
      return `${role}凭证已被质量暂停，禁止所有复测。仅管理员手动解除后才可再次检测。`;
    if (target.manual_disabled) return `${role}凭证已人工禁用，请先处理人工禁用状态。`;
  }
  return '';
}

export function holdsPause(rule, targets) {
  const config = rule.config || rule;
  const references = [
    config.auth_id,
    config.control_auth_id,
    config.judge?.mode === 'llm' ? config.judge.auth_id : '',
    rule.blocked_auth_id,
  ];
  return (
    rule.status === 'awaiting_release' ||
    !!rule.action_id ||
    references.some((id) => targets.find((target) => targetID(target) === id)?.quality_paused)
  );
}

export function safePreview(raw) {
  if (raw.length > 1024 * 1024) throw new Error('预览超过 1 MiB 上限。');
  const fenced = raw.trim().match(/^```(?:html|svg|xml)?\s*\n([\s\S]*?)\n```$/i);
  const parsed = new DOMParser().parseFromString(fenced ? fenced[1] : raw, 'text/html');
  parsed
    .querySelectorAll('script,iframe,frame,frameset,object,embed,base,meta,link,form')
    .forEach((node) => node.remove());
  parsed.querySelectorAll('a,area,map').forEach((node) => node.replaceWith(...node.childNodes));
  parsed.querySelectorAll('animate,set,animateTransform,animateMotion').forEach((node) => {
    if (
      /^(?:href|xlink:href|src|srcset|action|formaction|target)$/i.test(
        node.getAttribute('attributeName') || '',
      )
    )
      node.remove();
  });
  for (const node of parsed.querySelectorAll('*')) {
    for (const attribute of [...node.attributes]) {
      if (
        /^on/i.test(attribute.name) ||
        [
          'src',
          'srcset',
          'href',
          'xlink:href',
          'action',
          'formaction',
          'ping',
          'poster',
          'background',
          'srcdoc',
        ].includes(attribute.name.toLowerCase())
      )
        node.removeAttribute(attribute.name);
    }
  }
  return `<!doctype html><html><head><meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'none'; connect-src 'none'; img-src data:; media-src 'none'; style-src 'unsafe-inline'; font-src 'none'; frame-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'"><meta name="referrer" content="no-referrer"><style>body{margin:16px;font-family:system-ui}svg{max-width:100%}</style>${parsed.head.innerHTML}</head><body>${parsed.body.innerHTML}</body></html>`;
}
// Evidence rendering keeps execution, verdict, and comparability independent.
const evidenceValue = (value, fallback = '未知') =>
  value === null || value === undefined || value === '' ? fallback : String(value);
const evidenceBoolean = (value, yes, no) => value === true ? yes : value === false ? no : '未知';
const evidenceFact = (label, value) =>
  `<div><dt>${escapeHTML(label)}</dt><dd>${escapeHTML(evidenceValue(value))}</dd></div>`;
const evidenceFacts = (pairs) => `<dl class="evidence-facts">${pairs.map(([label, value]) => evidenceFact(label, value)).join('')}</dl>`;
const evidenceJSON = (object) => `<details class="raw-evidence"><summary>原始 JSON</summary><pre>${escapeHTML(JSON.stringify(object, null, 2))}</pre></details>`;
const judgeName = (mode) => ({ deterministic: '题型专用确定性判题', manual: '仅人工评审', llm: '独立模型判题' })[mode] || evidenceValue(mode);
const reviewName = (verdict) => ({ correct: '正确', incorrect: '错误', unknown: '无法判断' })[verdict] || '未评审';
const completedSample = (sample) => !!sample.execution_status && !['pending', 'queued', 'running', 'cancelling', 'canceling'].includes(sample.execution_status);

export function formatSampleUsage(sample) {
  const usage = sample.usage;
  if (!usage || sample.usage_known === false) return '用量未知';
  const total = usage.total_tokens ??
    (usage.input_tokens != null && usage.output_tokens != null ? usage.input_tokens + usage.output_tokens : null);
  return total == null || !Number.isFinite(Number(total)) || Number(total) < 0 ? '用量未知' : `${Number(total).toLocaleString('zh-CN')} token`;
}

function comparabilityTag(sample) {
  return sample.comparable === true ? tag('条件可比较', 'neutral') :
    sample.comparable === false ? tag('证据不足', 'warn') : tag('可比较性未知', 'neutral');
}

function sampleStates(sample) {
  const status = sample.execution_status;
  const tone = ['failed', 'error'].includes(status) ? 'bad' :
    ['running', 'queued', 'incomplete', 'refused', 'empty'].includes(status) ? 'warn' : 'neutral';
  return `<span class="sample-state"><span class="sample-state-label">执行</span>${tag(statusLabels[status] || status || '未知', tone)}</span>
    <span class="sample-state"><span class="sample-state-label">质量</span>${verdictTag(sample.quality_verdict)}</span>
    <span class="sample-state"><span class="sample-state-label">证据</span>${comparabilityTag(sample)}</span>`;
}

function renderFrozenConditions(run) {
  const config = run.config || {};
  const question = config.question || {};
  const parameters = config.parameters || {};
  const judge = config.judge || {};
  const questionName = question.title || config.question_id;
  const promptText = question.prompt || config.prompt;
  if (run.source === 'manual_test' && judge.mode === 'manual') {
    return `${evidenceFacts([
      ['凭证', config.auth_id], ['模型', config.models?.join(' / ')],
      ['题目', questionName], ['回答数量', config.samples],
      ['思考强度', parameters.reasoning_effort || '未指定'],
      ['输出上限', parameters.max_tokens == null ? null : `${parameters.max_tokens} token`],
      ['Temperature', parameters.temperature], ['题目版本', question.version],
    ])}<details class="evidence-section"><summary>查看本次题面</summary><pre class="condition-text">${escapeHTML(evidenceValue(promptText, '题面未记录'))}</pre></details>`;
  }
  return `<div class="condition-overview">${evidenceFacts([
    ['目标凭证', config.auth_id], ['提供方', config.provider],
    ['检测模型', config.models?.join(' / ')], ['每模型样本', config.samples],
    ['题目', questionName], ['题目版本', question.version],
    ['判题方式', judgeName(judge.mode)], ['判题模板', judge.template_version],
    ['输出上限', parameters.max_tokens == null ? null : `${parameters.max_tokens} token`],
    ['Temperature', parameters.temperature],
    ['推理强度', parameters.reasoning_effort || '未指定'],
    ['流式请求', evidenceBoolean(parameters.stream, '启用', '关闭')],
  ])}</div>
  <details class="evidence-section"><summary>冻结题面与参考标准</summary>
    <h4>题面</h4><pre class="condition-text">${escapeHTML(evidenceValue(promptText, '题面未记录'))}</pre>
    <h4>参考标准 / 参考答案</h4><pre class="condition-text">${escapeHTML(evidenceValue(config.review_standard || question.answer, '未提供；不能据此推断答案正确'))}</pre>
    <p class="field-note">参考标准只用于人工或独立判题，不发送给被测模型。</p>
  </details>
  <details class="evidence-section"><summary>身份、对照与配置版本</summary>${evidenceFacts([
    ['凭证身份版本', config.identity_version], ['冻结指纹', config.fingerprint],
    ['最终载荷配置版本', config.payload_config_version], ['批次', config.batch_id],
    ['对照凭证', config.control_auth_id || '未配置'], ['对照身份', config.control_identity],
    ['判题凭证', judge.auth_id || '未配置'], ['判题模型', judge.model || '未配置'],
    ['判题身份', config.judge_identity], ['随机种子', config.seed ?? question.seed],
    ['来源规则', config.rule_id || '无'], ['规则版本', config.rule_revision],
  ])}</details>
  ${evidenceJSON({ config, source: run.source, action_status: run.action_status, error: run.error || null })}`;
}

export function renderSampleEvidence(sample = {}) {
  const evidence = sample.evidence || {};
  const copy = { ...sample };
  delete copy.output;
  return `<div class="evidence-verdict sample-statuses">${sampleStates(sample)}</div>
    ${evidenceFacts([
      ['判题 / 执行说明', reasonLabel(sample.reason || sample.error)],
      ['可比较性说明', evidence.reason ? reasonLabel(evidence.reason) : sample.comparable === true ? '已记录可比较的执行条件' : '未确认完整、可比较的执行条件'],
    ])}
    <p class="field-note">执行完成不等于回答正确。只有完整、可比较且明确答错的证据才可能推进质量失败判定。</p>
    <h3>模型与执行路径</h3>${evidenceFacts([
      ['选择的模型', sample.model || sample.request_model],
      ['请求模型', evidence.requested_model], ['宿主解析模型', evidence.resolved_model],
      ['上游响应标记', sample.response_model], ['固定凭证', evidence.auth_id], ['执行路径', evidence.path],
    ])}<p class="field-note">response_model 仅是上游返回标记，不构成模型身份认证。</p>
    <h3>可比较性与最终条件</h3>${evidenceFacts([
      ['可比较', evidenceBoolean(sample.comparable, '是', '否')],
      ['载荷证据已验证', evidenceBoolean(evidence.verified, '是', '否')],
      ['条件发生变化', evidenceBoolean(evidence.conditions_changed, '是', '否')],
      ['最终载荷配置版本', evidence.payload_config_version],
    ])}
    <h3>耗时与用量</h3>${evidenceFacts([
      ['总耗时', duration(sample.total_ms)], ['首字时间（TTFT）', duration(sample.ttft_ms)],
      ['模型用量', formatSampleUsage(sample)],
      ['独立判题用量', formatSampleUsage({ usage: sample.judge_usage })],
      ['人工评审', reviewName(sample.review)], ['评审备注', sample.review_note || '无'],
    ])}<details class="evidence-section"><summary>诊断标识与原始说明</summary>${evidenceFacts([
      ['最终载荷指纹', evidence.payload_fingerprint],
      ['原始条件说明', evidence.reason], ['原始判题 / 执行说明', sample.reason || sample.error],
    ])}</details>${evidenceJSON(copy)}`;
}

function renderAnswer(sample, arrayIndex, ordinal, isActive, control = false, simple = false) {
  const model = sample.model || sample.request_model || '模型未知';
  const output = sample.output || '';
  const isHTML = /<(?:html|svg|!doctype)/i.test(output);
  const isCode = isHTML || /^\s*(?:```|~~~|\/\/|#!|(?:const|let|var|function|import|class|def|package)\s|<\?xml)/m.test(output);
  const pending = ['pending', 'queued', 'running'].includes(sample.execution_status);
  const emptyText = pending ? '等待完整回答…' : '未取得回答内容。';
  const review = sample.review || 'unknown';
  const actions = control ? '' : `<div class="sample-actions" data-part="actions">
    ${output.length > 1500 || output.split("\n").length > 12 ? `<button class="text-button" data-expand-sample="${arrayIndex}" aria-expanded="false">展开完整回答</button>` : ''}
    <button class="text-button" data-copy-sample="${arrayIndex}" ${!output ? 'disabled' : ''}>复制原文</button>
    ${isHTML ? `<button class="text-button" data-preview-sample="${arrayIndex}">预览 HTML / SVG</button>` : ''}
    <button class="text-button" data-evidence-sample="${arrayIndex}">${simple ? '执行详情' : '条件详情'}</button>
  </div>`;
  const reviewHTML = simple ? (sample.review ? `<details class="review-disclosure" data-part="review"><summary>历史人工评审</summary><p>${escapeHTML(reviewName(sample.review))}${sample.review_note ? ` · ${escapeHTML(sample.review_note)}` : ''}</p></details>` : '') : control ? `<details class="evidence-section" data-part="review"><summary>对照条件详情</summary>${renderSampleEvidence(sample)}</details>` :
    `<details class="review-disclosure" data-part="review"><summary>人工评审${sample.review ? ` · ${escapeHTML(reviewName(sample.review))}` : ''}</summary>
      <p class="field-note">评审单独留存，不触发自动暂停或解除。</p>
      <div class="sample-review"><label>结论<select aria-label="样本 ${arrayIndex + 1} 人工评审" id="review-${arrayIndex}">
        <option value="unknown" ${review === 'unknown' ? 'selected' : ''}>无法判断</option>
        <option value="correct" ${review === 'correct' ? 'selected' : ''}>正确</option>
        <option value="incorrect" ${review === 'incorrect' ? 'selected' : ''}>错误</option></select></label>
        <label>评审备注<input id="review-note-${arrayIndex}" maxlength="1000" placeholder="记录判断依据" aria-label="评审备注" value="${escapeHTML(sample.review_note || '')}"></label>
        <button class="subtle small" data-review-sample="${arrayIndex}" ${isActive ? 'disabled' : ''}>保存评审</button>
      </div>${isActive ? '<p class="field-note">任务结束后可保存人工评审。</p>' : ''}
    </details>`;
  const reason = sample.error || (!simple || sample.reason !== 'manual assessment required' ? sample.reason : '');
  return `<article class="sample${control ? ' control-sample' : ''}" data-key="${escapeHTML(`${control ? 'control-' : ''}sample:${JSON.stringify([model, ordinal])}`)}" data-model="${escapeHTML(model)}" data-sample="${ordinal}">
    <div class="sample-heading" data-part="heading"><span class="sample-model-label" title="${escapeHTML(model)}">${escapeHTML(model)}</span><h4>${control ? '对照 · ' : ''}样本 ${ordinal + 1}</h4><div class="sample-statuses">${simple ? statusTag(sample.execution_status) : sampleStates(sample)}</div></div>
    <pre class="answer-text ${isCode ? 'answer-code' : 'answer-prose'}" data-part="output" tabindex="0" aria-label="${escapeHTML(model)} 样本 ${ordinal + 1} 回答">${escapeHTML(output || emptyText)}</pre>
    <p class="sample-reason${reason ? '' : ' hidden'}" data-part="reason">${reason ? escapeHTML(reasonLabel(reason)) : ''}</p>
    <div class="sample-meta" data-part="meta"><span>耗时 ${duration(sample.total_ms)}</span><span>TTFT ${duration(sample.ttft_ms)}</span><span>${escapeHTML(formatSampleUsage(sample))}</span></div>
    ${actions}${reviewHTML}</article>`;
}

export function renderRunEvidence(run = {}) {
  const config = run.config || {};
  const samples = run.samples || [];
  const simple = run.source === 'manual_test' && config.judge?.mode === 'manual';
  const isActive = active(run);
  const models = [...new Set([...(config.models || []), ...samples.map((sample) => sample.model || sample.request_model || '模型未知')])];
  const count = Number.isInteger(config.samples) && config.samples > 0 ? config.samples : 1;
  const planned = models.length * count;
  const completed = samples.filter(completedSample).length;
  const comparable = samples.filter((sample) => sample.comparable === true).length;
  const unknown = samples.filter((sample) => !['correct', 'incorrect'].includes(sample.quality_verdict)).length;
  const metaHTML = simple
    ? `<div class="run-state-line">${statusTag(run.status)}<strong class="run-model" title="${escapeHTML(models.join(' / '))}">${escapeHTML(models.join(' / '))}</strong><span>${completed} / ${planned} 份回答已结束 · 不自动评分</span><span>${escapeHTML(config.question?.title || config.question_id || '题目测试')}</span></div>${run.error || run.block_reason ? `<p class="notice error">${escapeHTML(run.error || run.block_reason)}</p>` : ''}`
    : `<div class="run-state-line">${statusTag(run.status)}<strong>${completed} / ${planned} 样本已结束</strong><span>${comparable} 份可比较 · ${unknown} 份无法判断</span></div>
    <div class="run-context-line"><span>凭证 <strong>${escapeHTML(evidenceValue(config.auth_id))}</strong></span><span>${escapeHTML(config.question?.title || config.question_id || '题目未知')}</span><span>动作：${escapeHTML(statusLabels[run.action_status] || run.action_status || '未知')}</span></div>
    ${run.error || run.block_reason ? `<p class="notice error">${escapeHTML(run.error || run.block_reason)}</p>` : ''}`;
  const groups = models.map((model) => {
    const entries = samples.map((sample, index) => ({ sample, index })).filter(({ sample }) => (sample.model || sample.request_model || '模型未知') === model);
    const content = Array.from({ length: count }, (_, ordinal) => {
      const entry = entries.find(({ sample }, localIndex) => (sample.index ?? localIndex) === ordinal);
      if (entry) return renderAnswer(entry.sample, entry.index, ordinal, isActive, false, simple);
      return `<article class="sample sample-pending" data-key="${escapeHTML(`sample:${JSON.stringify([model, ordinal])}`)}" data-model="${escapeHTML(model)}" data-sample="${ordinal}"><div class="sample-heading" data-part="heading"><span class="sample-model-label" title="${escapeHTML(model)}">${escapeHTML(model)}</span><h4>样本 ${ordinal + 1}</h4>${tag(isActive ? '等待结果' : '样本缺失', isActive ? 'neutral' : 'warn')}</div><p class="empty-line" data-part="pending">${isActive ? '后台按计划执行，结果取得后在此更新。' : simple ? '未取得这份回答，请查看执行状态。' : '本轮未取得该样本，不能作为完整失败证据。'}</p></article>`;
    }).join('');
    const extras = entries.filter(({ sample }, localIndex) => (sample.index ?? localIndex) >= count).map(({ sample, index }) => renderAnswer(sample, index, sample.index ?? index, isActive, false, simple)).join('');
    return `<section class="model-result" data-key="${escapeHTML(`model:${model}`)}" data-model="${escapeHTML(model)}"><div class="model-heading" data-part="model-heading"><div class="model-identity"><h3 title="${escapeHTML(model)}">${escapeHTML(model)}</h3><span>${entries.length} / ${count} 份结果</span></div><button type="button" class="text-button model-copy" data-copy-model="${escapeHTML(model)}" aria-label="复制模型 ID ${escapeHTML(model)}">复制 ID</button></div>${content}${extras}</section>`;
  }).join('');
  const controlHTML = run.controls?.length ? `<details class="control-results" data-key="controls"><summary>同批对照 · ${run.controls.length} 份结果</summary><p class="field-note">只有身份不同、同题、同模型映射、同有效条件的通过样本才构成有效对照。</p><div class="comparison-grid">${run.controls.map((sample, index) => `<section class="model-result" data-key="${escapeHTML(`control-model:${index}:${sample.model || '模型未知'}`)}"><div class="model-heading" data-part="model-heading"><h3>${escapeHTML(sample.model || '模型未知')}</h3><span>对照凭证</span></div>${renderAnswer(sample, index, sample.index ?? index, isActive, true)}</section>`).join('')}</div></details>` : '';
  return {
    metaHTML,
    conditionsHTML: renderFrozenConditions(run),
    samplesHTML: `<div class="comparison-grid${simple ? ` question-test-results${models.length === 1 ? ' single-question' : ''}` : ''}" data-key="models">${groups || '<p class="empty-line">本轮没有记录模型或样本。</p>'}</div>${controlHTML}`,
    completed,
    planned,
  };
}
// Pure operation views keep restriction ownership separate from rule state.
const restrictionLabels = {
  manual_disabled: '人工禁用',
  quality_pause: '质量暂停',
  quality_paused: '质量暂停',
  native_cooldown: '原生冷却',
  cooldown: '原生冷却',
  unavailable: '当前不可用',
};
const restrictionText = (reason) => restrictionLabels[reason] || String(reason ?? '未知限制');
const operationFact = (label, value) =>
  `<div><dt>${escapeHTML(label)}</dt><dd>${escapeHTML(value == null || value === '' ? '未知' : value)}</dd></div>`;
const restrictionList = (reasons, fallback) =>
  Array.isArray(reasons) && reasons.length
    ? `<ul class="restriction-list">${reasons.map((reason) => `<li>${escapeHTML(restrictionText(reason))}</li>`).join('')}</ul>`
    : `<p class="field-note">${escapeHTML(fallback)}</p>`;

export function renderRuleRows(rules, targets, { connected, targetsFresh }) {
  if (!rules.length) return '<tr><td colspan="5" class="empty-line">还没有检测规则。新建只观察规则，先积累可比较的证据。</td></tr>';
  return rules.map((rule) => {
    const config = rule.config || rule;
    const held = holdsPause(rule, targets);
    const ready = connected && targetsFresh;
    const enabled = rule.enabled === true && !held;
    const label = held ? '等待人工解除' : rule.enabled === true ? '检测规则启用' : rule.enabled === false ? '检测规则已暂停' : '规则状态未知';
    const blockedBy = held ? (rule.blocked_auth_id ? `受暂停凭证 ${rule.blocked_auth_id} 影响` : '目标或依赖凭证暂停，所有检测停止') : !ready ? '凭证状态未确认，请刷新' : '';
    const id = escapeHTML(rule.id);
    const schedule = !enabled ? '已停止检测' : rule.scheduling_enabled ? (rule.cron || `每 ${rule.interval_seconds ?? '未知'} 秒`) : '仅手动执行';
    return `<tr data-key="rule:${id}">
      <td data-label="规则 / 目标"><div class="rule-info"><strong>${escapeHTML(rule.name || rule.id)}</strong><small>${escapeHTML(config.auth_id || '目标未知')}</small><details><summary>${escapeHTML(config.models?.length ?? 0)} 个检测模型 · 条件</summary><p>${escapeHTML(config.models?.join(' / ') || '模型未知')}</p><dl class="evidence-facts">${operationFact('题目', config.question_id)}${operationFact('每模型样本', config.samples)}${operationFact('规则版本', rule.revision)}${operationFact('对照凭证', config.control_auth_id || '未配置')}${operationFact('判题', config.judge?.mode === 'llm' ? `独立模型 · ${config.judge.auth_id || '未知'} · ${config.judge.model || '未知'}` : config.judge?.mode === 'deterministic' ? '题型专用确定性判题' : config.judge?.mode === 'manual' ? '人工评审' : '未知')}</dl></details></div></td>
      <td data-label="检测与处置">${tag(label, held ? 'warn' : enabled ? 'good' : 'neutral')}<small>${rule.action === 'pause_credential' ? '允许配置凭证暂停' : rule.action === 'observe_only' ? '只观察' : '处置未知'}<br>规则自动暂停许可：${rule.automatic_pause === true ? '开' : rule.automatic_pause === false ? '关' : '未知'}<br>同时受全局自动暂停许可约束</small>${blockedBy ? `<small class="error-note">${escapeHTML(blockedBy)}</small>` : ''}</td>
      <td data-label="调度">${escapeHTML(schedule)}<small>${escapeHTML(rule.timezone || '时区未知')}${enabled && rule.scheduling_enabled ? `<br>下次 ${escapeHTML(date(rule.next_run_at || rule.next_run))}` : ''}</small>${enabled && rule.scheduling_enabled ? '<small>同时受全局调度许可约束</small>' : ''}</td>
      <td data-label="连续失败"><strong>${escapeHTML(rule.fail_count ?? '未知')} / ${escapeHTML(rule.fail_threshold ?? '未知')}</strong><small>完整、可比较且明确答错的轮次</small><details><summary>计数口径</summary><p>所选检测模型全部失败才推进计数；未知、缺样、条件变化或混合结果打断连续证据。网络、鉴权、限流等执行错误不算答错。</p></details></td>
      <td data-label="操作"><div class="button-row"><button class="subtle small" data-rule-run="${id}" ${!ready || !enabled ? 'disabled' : ''}>立即执行</button><button class="subtle small" data-rule-pause="${id}" ${held || !ready || rule.enabled == null ? 'disabled' : ''}>${held ? '等待人工解除' : rule.enabled ? '暂停检测规则' : '恢复检测规则'}</button><button class="text-button" data-rule-edit="${id}">编辑</button><button class="text-button" data-rule-delete="${id}" ${held || !ready ? 'disabled' : ''}>删除</button></div></td>
    </tr>`;
  }).join('');
}

export function renderTargetRows(targets) {
  if (!targets.length) return '<tr><td colspan="5" class="empty-line">没有可展示的凭证。请连接并刷新；列表缺项不表示原有暂停已解除。</td></tr>';
  return targets.map((target) => {
    const reasons = target.reasons ?? target.unavailable_reasons;
    return `<tr data-key="target:${escapeHTML(targetID(target))}">
      <td data-label="凭证"><strong>${escapeHTML(target.provider || '未知')} · ${escapeHTML(target.auth_index || targetID(target) || '未知')}</strong><small>${escapeHTML(targetID(target) || '标识未知')}<br>${escapeHTML(target.type || '类型未知')}</small></td>
      <td data-label="人工禁用">${tag(target.manual_disabled === true ? '人工禁用' : target.manual_disabled === false ? '未禁用' : '未知', target.manual_disabled ? 'bad' : 'neutral')}</td>
      <td data-label="质量暂停">${tag(target.unsupported_reason === 'official_host_quality_pause_unavailable' ? '官方接口不支持' : target.quality_paused === true ? '整个凭证已暂停' : target.quality_paused === false ? '无质量暂停' : '未知', target.quality_paused ? 'warn' : 'neutral')}</td>
      <td data-label="有效状态 / 原因">${tag(target.effective_available === true ? '可用' : target.effective_available === false ? '不可用' : '状态未知', target.effective_available === true ? 'good' : 'neutral')}${restrictionList(reasons, Array.isArray(reasons) ? '未报告其他限制' : '限制原因未知')}</td>
      <td data-label="能力与范围"><small>${target.can_pause === true ? '支持凭证级质量暂停' : target.can_pause === false ? '不支持质量暂停' : '暂停能力未知'}<br>${escapeHTML(target.unsupported_reason === 'official_host_quality_pause_unavailable' ? '支持模型测试与观察' : target.unsupported_reason || '暂停后仅人工解除')}</small></td>
    </tr>`;
  }).join('');
}

export function renderActionItems(actions, targets) {
  if (!actions.length) return '<div class="empty-line calm-empty"><strong>没有已加载的质量暂停动作</strong><span>在“凭证限制”中核对当前可用状态。</span></div>';
  return actions.map((action, index) => {
    const target = targets.find((entry) => targetID(entry) === action.auth_id);
    const owned = action.owner_plugin === 'model-quality';
    return `<article class="action-item" data-key="action:${escapeHTML(action.action_id || action.id || action.auth_id)}">
      <div class="action-summary"><div class="button-row">${tag('质量暂停', 'warn')}<strong>${escapeHTML(target?.auth_index || action.auth_id || '凭证未知')}</strong></div><p>${escapeHTML(action.reason ? reasonLabel(action.reason) : '暂停原因未提供')}</p><small>影响整个凭证所有模型的新业务执行 · ${escapeHTML(date(action.created_at || action.time))}</small><dl class="evidence-facts">${operationFact('暂停归属', action.owner_plugin)}${operationFact('检测规则', action.rule_id)}${operationFact('动作版本', action.action_revision)}</dl></div>
      <div class="action-decision">${action.evidence_run_id ? `<button class="text-button" data-run="${escapeHTML(action.evidence_run_id)}">查看失败证据</button>` : ''}<p>${target?.manual_disabled ? '同时存在人工禁用；解除质量暂停后仍然生效。' : target ? '解除后相关检测规则仍停止，须另行恢复。' : '凭证当前不可发现；解除前需核对身份与剩余限制。'}</p><button class="danger small" data-action-preview="${index}" ${!owned || !(action.action_id || action.id) || action.action_revision == null ? 'disabled' : ''}>预览并解除自身暂停</button>${!owned ? '<small>仅可解除本插件持有的暂停</small>' : ''}</div>
    </article>`;
  }).join('');
}

export function renderReleasePreview(preview, originalAction = {}) {
  const action = preview.action || originalAction;
  const targets = Array.isArray(preview.targets) ? preview.targets : preview.targets?.items || [];
  const target = targets.find((entry) => targetID(entry) === action.auth_id);
  const hasRemaining = Array.isArray(preview.remaining_reasons);
  const currentReasons = target?.reasons ?? target?.unavailable_reasons;
  const knownRemaining = hasRemaining ? [...preview.remaining_reasons] : (Array.isArray(currentReasons) ? currentReasons.filter((reason) => !['quality_pause', 'quality_paused'].includes(reason)) : []);
  if (target?.manual_disabled && !knownRemaining.includes('manual_disabled')) knownRemaining.push('manual_disabled');
  const conflict = preview.conflict === true || preview.blocked === true || preview.can_release === false;
  return `${conflict ? '<p class="notice error">当前暂停无法解除。状态或版本已变化，请关闭并刷新后重新预览。</p>' : ''}
    <section class="release-scope"><h3>${escapeHTML(target?.auth_index || action.auth_id || '影响凭证未知')}</h3><p>仅释放下面这一条归属匹配的质量暂停，作用范围为整个凭证的所有模型。</p><dl class="evidence-facts">${operationFact('凭证标识', action.auth_id)}${operationFact('暂停归属', action.owner_plugin)}${operationFact('关联规则', action.rule_id)}${operationFact('动作标识', action.action_id || action.id)}${operationFact('动作版本', action.action_revision)}${operationFact('凭证身份版本', action.identity_version)}</dl></section>
    <section class="release-scope"><h3>解除后仍需处理</h3>${restrictionList(knownRemaining, hasRemaining ? '预览未报告其他限制；最终可用状态以解除后的宿主查询为准。' : '剩余限制尚不能完整确定，解除后须重新读取宿主状态。')}<p class="field-note">人工禁用、原生冷却与其他归属的暂停继续生效。当前质量暂停标记不能证明只有一个暂停归属。</p></section>
    <details><summary>原始解除预览 JSON</summary><pre>${escapeHTML(JSON.stringify(preview, null, 2))}</pre></details>`;
}
