// Presentation helpers have no access to management credentials or task state.
export const escapeHTML = (value) =>
  String(value ?? '').replace(
    /[&<>"']/g,
    (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c],
  );
export const items = (data) => (Array.isArray(data) ? data : data?.items || []);
export const targetID = (target) => target.auth_id || target.id;
// These are upstream rule-entry presets, not a claim of live CPA availability.
export const ruleModelShortcuts = ['gpt-6-astra', 'gpt-6.1-sol', 'gpt-5.6-sol'];
export const splitRuleModels = (text) => [...new Set(text.split(/[\s,，;；]+/u).filter(Boolean))];
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
const questionTitles = {
  'logic-candy': '糖果逻辑测试', 'logic-boxes': '约束盒子',
  'visual-pelican-html': '鹈鹕骑自行车（HTML）', 'visual-pelican': '鹈鹕骑自行车',
  'random-linear': '随机约束题', custom: '自定义题目',
};
export const questionTitle = (question, id = question?.id) =>
  question?.title || questionTitles[id] || id || '题目未记录';
const auditLabels = {
  disable_pending: '正在暂停凭证', delete_pending: '正在移除凭证', restore_pending: '正在恢复凭证',
  credential_suspended: '已暂停凭证', credential_removed: '已移除凭证', credential_restored: '整轮通过，已恢复凭证',
  credential_action_uncertain: '处置结果待核对', credential_external_enable: '已识别 CPA 人工启用',
  credential_external_change: '已识别 CPA 当前凭证变更', credential_handling_acknowledged: '人工处理已核对，规则已停止',

  scope_dispatched: '匹配动态检测范围',
  observation_completed: '观察完成', settings_saved: '设置已保存',
  rule_created: '规则已创建', rule_updated: '规则已更新', rule_saved: '规则已保存',
  rule_schedule_state: '规则调度已变更', rule_schedule_invalid: '检测周期无效，未执行', rule_deleted: '规则已删除',
  credential_login_prepare: '凭证登录前检查', legacy_ownership_retained: '历史暂停归属已保留',
  pause_requested: '已请求质量暂停', pause_uncertain: '质量暂停结果待核对',
  credential_paused: '凭证已质量暂停', manual_release_requested: '已请求人工解除',
  manual_release: '已人工解除自身暂停', reconciled_pause: '暂停归属已核对',
};
export const auditLabel = (kind) => auditLabels[kind] || kind || '审计记录';
const reasonLabels = {
  cancelled: '测试已取消',
  'context canceled': '测试已取消',
  queue_full: '队列容量不足，本轮未入队。',
  budget_exhausted: '调用预算已用尽，本轮未执行。',
  official_host_evidence_unavailable: '当前宿主未提供可验证的最终执行条件，本次结果仅供观察。',
  retained_pause_unverified: '历史暂停归属待核对',
  management_probe_not_business_payload: '本次使用 CPA 管理接口检测指定凭证。',
  'exact answer': '回答与标准答案一致',
  'integer differs from frozen reference': '回答与标准答案不一致',
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
  unavailable: '记录不可用',
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
  observe_only: '仅检测',
  credential_suspended: '已暂停凭证',
  credential_kept_disabled: '保持停用',
  credential_removed: '已移除凭证',
  credential_restored: '整轮通过，已恢复',
  credential_action_uncertain: '处置结果待核对',
  credential_action_attention: '凭证处置待核对',
  insufficient_evidence: '整轮判定依据不足，未处理凭证',
  catalog_action_unconfirmed: '目录或凭证资格未能再次确认，未执行处置',
  credential_changed: '凭证已变更，未处理',
  management_session_required: '管理会话待连接',
  disable_pending: '正在暂停凭证',
  delete_pending: '正在移除凭证',
  restore_pending: '正在恢复凭证',
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
    if (target.quality_paused || target.retained_pause)
      return `${role}凭证已被质量暂停，禁止所有复测。仅管理员手动解除后才可再次检测。`;
    if (target.manual_disabled) return `${role}凭证已人工禁用，请先处理人工禁用状态。`;
    if (target.effective_available === false) return `${role}凭证当前不可用。`;
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
    !!rule.action_id || !!rule.blocked_auth_id ||
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
const judgeName = (mode) => ({ deterministic: '标准答案判定', manual: '人工评审', llm: '独立模型判题' })[mode] || evidenceValue(mode);
const reviewName = (verdict) => ({ correct: '正确', incorrect: '错误', unknown: '无法判断' })[verdict] || '未评审';
const completedSample = (sample) => !!sample.execution_status && !['pending', 'queued', 'running', 'cancelling', 'canceling'].includes(sample.execution_status);
const sampleModel = sample => sample.model || sample.request_model || '模型未知';

function answerVerdict(sample) {
  if (sample.execution_status !== 'completed' || sample.error) return '';
  return sample.review || (['correct', 'incorrect'].includes(sample.quality_verdict) ? sample.quality_verdict : sample.answer_match) || 'unknown';
}

function answerTag(sample) {
  const verdict = answerVerdict(sample);
  if (!verdict) return statusTag(sample.error && sample.execution_status === 'completed' ? 'failed' : sample.execution_status);
  if (verdict === 'correct') return tag('答案正确', 'good');
  if (verdict === 'incorrect') return tag('答案错误', 'warn');
  return tag('未判定');
}

function answerCounts(run) {
  const counts = new Map();
  for (const sample of run.samples || []) {
    if (!completedSample(sample)) continue;
    const key = answerVerdict(sample) || 'no_answer';
    counts.set(key, (counts.get(key) || 0) + 1);
  }
  return counts;
}

export function renderManualVerdict(run) {
  if (run.source !== 'manual_test' || active(run)) return '';
  const counts = answerCounts(run);
  const correct = counts.get('correct') || 0, incorrect = counts.get('incorrect') || 0;
  if (correct && incorrect) return tag('部分正确', 'warn');
  if (correct || incorrect) {
    const models = run.config?.models || [];
    const count = run.config?.samples;
    const reviewed = run.samples.filter(s => ['correct', 'incorrect'].includes(answerVerdict(s)));
    const positions = new Set(reviewed.filter(s => models.includes(sampleModel(s)) && Number.isInteger(s.index) && s.index >= 0 && s.index < count)
      .map(s => JSON.stringify([sampleModel(s), s.index])));
    if (counts.size > 1 || !models.length || !count || reviewed.length !== positions.size || positions.size !== models.length * count) return tag('部分已判定');
    return correct ? tag('正确', 'good') : tag('错误', 'warn');
  }
  return counts.has('unknown') ? tag('未判定') : '';
}

export function formatSampleUsage(sample) {
  const usage = sample.usage;
  if (!usage || sample.usage_known === false) return '用量未知';
  const total = usage.total_tokens ??
    (usage.input_tokens != null && usage.output_tokens != null ? usage.input_tokens + usage.output_tokens : null);
  return total == null || !Number.isFinite(Number(total)) || Number(total) < 0 ? '用量未知' : `${Number(total).toLocaleString('zh-CN')} token`;
}

function qualityCounts(samples) {
  const correct = samples.filter(s => s.quality_verdict === 'correct').length;
  const incorrect = samples.filter(s => s.quality_verdict === 'incorrect').length;
  return { correct, incorrect, unknown: samples.length - correct - incorrect };
}

function credentialHandlingLabel(run) {
  if (run.status === 'skipped' && run.skip_detail?.code === 'error_policy_disabled') return '保持暂停，本轮未改写';
  if (run.action_status === 'observe_only') return active(run) ? '检测中，尚未处理' : '未处理凭证';
  if (run.action_status === 'credential_action_uncertain') {
    const intent = { disable_credential: '停用', delete_credential: '移除', restore_credential: '恢复' }[run.credential_action];
    if (intent) return `${intent}结果待核对`;
  }
  return statusLabels[run.action_status] || run.action_status || '未记录处理结果';
}

export const upstreamConditions = Object.freeze({
  unauthorized: '401 认证失败', payment_required: '402 付费要求',
  quota_exhausted: '429 配额耗尽', model_unsupported: '指定模型不支持',
});

export const credentialActions = Object.freeze({ observe_only: '不处理', disable_credential: '暂停凭证', delete_credential: '移除凭证' });
export function handlesCredential(action) {
  return action === 'disable_credential' || action === 'delete_credential';
}
export function configuredUpstreamActions(actions) {
  return Object.entries(upstreamConditions)
    .filter(([key]) => handlesCredential(actions?.[key]))
    .map(([key, label]) => [label, credentialActions[actions[key]]]);
}
const upstreamLabel = key => key === 'model_unsupported' ? '400/404 指定模型不支持' : Object.hasOwn(upstreamConditions, key) ? upstreamConditions[key] : key;

function admissionEvidence(run) {
  if (run.samples?.length || run.pruned_samples) return null;
  const preflight = run.preflight_detail, detail = preflight || run.skip_detail;
  const catalogModel = run.config?.catalog_missing_model;
  if (!detail && !catalogModel && run.status !== 'skipped') return null;
  const catalog = detail?.code === 'missing_model' ? !!(preflight || catalogModel) : !detail && !!catalogModel;
  const suspended = detail?.code === 'error_policy_disabled';
  const stopped = catalog && (run.block_reason || run.error ||
    (run.cancel_requested || run.status === 'cancelled' ? 'cancelled' : ''));
  const catalogResult = catalog && !stopped;
  const checked = catalogResult && ['completed', 'skipped'].includes(run.status);
  return {
    id: detail?.auth_id || run.config?.auth_id, email: detail?.email || run.credential_email,
    model: detail?.model || catalogModel,
    status: checked ? tag('已检测') : statusTag(run.status),
    result: catalogResult ? '模型不支持（CPA 目录）' : suspended ? '已按错误规则暂停，未重复检测' : '未发起模型请求',
    explanation: catalogResult ? '目录检查已确认不支持所选模型，未发起答题。' : suspended ? '本轮未重复答题。' : '未发起模型请求',
    source: catalog || detail?.code === 'missing_model' ? 'CPA 凭证模型目录' : suspended ? '已确认的错误策略暂停' : preflight ? '当前凭证状态检查' : '范围调度准入',
    reasonLabel: stopped ? '未执行原因' : catalog || preflight ? '检查结果' : '跳过原因',
    reason: stopped ? reasonLabel(stopped) : detail?.reason || (catalog ? '凭证目录中缺少所选模型。' : '原因未记录'),
  };
}

function sampleStates(sample) {
  const status = sample.execution_status;
  const tone = ['failed', 'error'].includes(status) ? 'bad' :
    ['running', 'queued', 'incomplete', 'refused', 'empty'].includes(status) ? 'warn' : 'neutral';
  return `<span class="sample-state"><span class="sample-state-label">执行</span>${tag(statusLabels[status] || status || '未知', tone)}</span>
    <span class="sample-state"><span class="sample-state-label">答案</span>${verdictTag(sample.quality_verdict)}</span>`;
}

function renderFrozenConditions(run, credentialLabel, credentialName) {
  const config = run.config || {};
  const question = config.question || {};
  const parameters = config.parameters || {};
  const judge = config.judge || {};
  const questionName = questionTitle(question, config.question_id);
  const promptText = question.prompt || config.prompt;
  if (run.source === 'manual_test' && judge.mode === 'manual') {
    return `${evidenceFacts([
      ['凭证', credentialLabel], ['模型', config.models?.join(' / ')],
      ['题目', questionName], ['回答数量', config.samples],
      ...(['logic', 'random'].includes(question.kind) && question.answer ? [['标准答案', question.answer]] : []),
      ['思考强度', parameters.reasoning_effort || '未指定'],
      ...(parameters.max_tokens == null ? [] : [['输出上限', `${parameters.max_tokens} token`]]),
      ...(parameters.temperature == null ? [] : [['Temperature', parameters.temperature]]),
    ])}<details class="evidence-section"><summary>查看本次题面</summary><pre class="condition-text">${escapeHTML(evidenceValue(promptText, '题面未记录'))}</pre></details>`;
  }
  return `<div class="condition-overview">${evidenceFacts([
    ['目标凭证', credentialLabel], ['提供方', config.provider],
    ['检测模型', config.models?.join(' / ')], ['每模型样本', config.samples],
    ['题目', questionName], ['判题方式', judgeName(judge.mode)],
    ['请求思考强度', parameters.reasoning_effort || '未指定'],
    ...(parameters.max_tokens == null ? [] : [['输出上限', `${parameters.max_tokens} token`]]),
    ...(parameters.temperature == null ? [] : [['Temperature', parameters.temperature]]),
    ['流式请求', evidenceBoolean(parameters.stream, '启用', '关闭')],
  ])}</div>
  <details class="evidence-section"><summary>本次题目与参考答案</summary>
    <h4>题面</h4><pre class="condition-text">${escapeHTML(evidenceValue(promptText, '题面未记录'))}</pre>
    <h4>参考标准 / 参考答案</h4><pre class="condition-text">${escapeHTML(evidenceValue(config.review_standard || question.answer, '未提供；不能据此推断答案正确'))}</pre>
    <p class="field-note">参考答案用于判题，不发送给被测模型。</p>
  </details>
  <details class="evidence-section"><summary>检测身份与关联记录</summary>${evidenceFacts([
    ['凭证身份版本', config.identity_version], ['冻结指纹', config.fingerprint],
    ['最终载荷配置版本', config.payload_config_version], ...(config.scope_batch_id ? [['范围批次', config.scope_batch_id]] : []), ...(config.batch_id ? [['对照批次', config.batch_id]] : []),
    ['对照凭证', credentialName(config.control_auth_id) || '未配置'], ['对照身份', config.control_identity],
    ['判题凭证', credentialName(judge.auth_id) || '未配置'], ['判题模型', judge.model || '未配置'],
    ['判题身份', config.judge_identity], ['随机种子', config.seed ?? question.seed],
    ['关联规则', config.rule_id || '无'], ['规则版本', config.rule_revision],
    ['判题模板', judge.template_version],
  ])}</details>
  ${evidenceJSON({ config, source: run.source, status: run.status, action_status: run.action_status, credential_action: run.credential_action, upstream_condition: run.upstream_condition, preflight_detail: run.preflight_detail, skip_detail: run.skip_detail, error: run.error || null })}`;
}

export function renderSampleEvidence(sample = {}, manual = false, credentialLabel = sample.evidence?.auth_id) {
  const evidence = sample.evidence || {};
  const copy = { ...sample };
  delete copy.output;
  return `<div class="evidence-verdict sample-statuses">${manual ? statusTag(sample.execution_status) + (answerVerdict(sample) ? answerTag(sample) : '') : sampleStates(sample)}</div>
    ${evidenceFacts([
      ['判题 / 执行说明', manual && sample.review ? '历史人工评审' : manual && ['correct', 'incorrect'].includes(sample.answer_match) ? '按本次题目的标准答案核对' : reasonLabel(sample.reason || sample.error)],
      ...(sample.upstream_condition ? [['上游错误', upstreamLabel(sample.upstream_condition)]] : []),
    ])}
    <h3>模型与执行路径</h3>${evidenceFacts([
      ['选择的模型', sample.model || sample.request_model],
      ['请求模型', evidence.requested_model], ['宿主解析模型', evidence.resolved_model],
      ['上游响应标记', sample.response_model], ['固定凭证', credentialLabel], ['执行路径', evidence.path],
    ])}<p class="field-note">上游返回的模型名称不构成身份认证。</p>
    <h3>耗时与用量</h3>${evidenceFacts([
      ['总耗时', duration(sample.total_ms)], ['首字时间（TTFT）', duration(sample.ttft_ms)],
      ['模型用量', formatSampleUsage(sample)],
      ['独立判题用量', formatSampleUsage({ usage: sample.judge_usage })],
      ['人工评审', reviewName(sample.review)], ['评审备注', sample.review_note || '无'],
    ])}<details class="evidence-section"><summary>执行条件与诊断标识</summary>${evidenceFacts([
      ['执行条件说明', evidence.reason ? reasonLabel(evidence.reason) : null],
      ['网关执行条件可比较', evidenceBoolean(sample.comparable, '是', '否')],
      ['网关载荷已验证', evidenceBoolean(evidence.verified, '是', '否')],
      ['条件发生变化', evidenceBoolean(evidence.conditions_changed, '是', '否')],
      ['最终载荷配置版本', evidence.payload_config_version],
      ['最终载荷指纹', evidence.payload_fingerprint],
      ['原始条件说明', evidence.reason], ['原始判题 / 执行说明', sample.reason || sample.error],
    ])}</details>${evidenceJSON(copy)}`;
}

function renderAnswer(sample, recordIndex, ordinal, isActive, { control, simple, qualityDetail, annotation = '' }) {
  const model = sampleModel(sample);
  const number = Number.isInteger(ordinal) && ordinal >= 0 ? ordinal + 1 : null;
  const label = `${control ? '对照 · ' : ''}${simple ? '回答' : '样本'}${number === null ? '（序号未知）' : ' ' + number}`;
  const output = sample.output || '';
  const isHTML = /<(?:html|svg|!doctype)/i.test(output);
  const isCode = isHTML || /^\s*(?:```|~~~|\/\/|#!|(?:const|let|var|function|import|class|def|package)\s|<\?xml)/m.test(output);
  const pending = ['pending', 'queued', 'running'].includes(sample.execution_status);
  const emptyText = pending ? '正在等待回答，取得结果后会在这里更新。' : '本次没有取得回答内容，请查看执行状态。';
  const review = sample.review || 'unknown';
  const idPrefix = qualityDetail ? 'quality-' : '';
  const statusHTML = qualityDetail ? '' : `<div class="sample-statuses">${simple ? answerTag(sample) : sampleStates(sample)}</div>`;
  const actions = control ? '' : `<div class="sample-actions" data-part="actions">
    ${output.length > 1500 || output.split("\n").length > 12 ? `<button class="text-button" data-expand-sample="${recordIndex}" aria-expanded="false">展开完整回答</button>` : ''}
    <button class="text-button" data-copy-sample="${recordIndex}" ${!output ? 'disabled' : ''}>复制原文</button>
    ${isHTML ? `<button class="text-button" data-preview-sample="${recordIndex}">预览 HTML / SVG</button>` : ''}
    <button class="text-button" data-evidence-sample="${recordIndex}">${simple ? '执行详情' : '条件详情'}</button>
  </div>`;
  const reviewHTML = simple ? (sample.review ? `<details class="review-disclosure" data-part="review"><summary>历史人工评审</summary><p>${escapeHTML(reviewName(sample.review))}${sample.review_note ? ` · ${escapeHTML(sample.review_note)}` : ''}</p></details>` : '') : control ? `<details class="evidence-section" data-part="review"><summary>对照条件详情</summary>${renderSampleEvidence(sample)}</details>` :
    `<details class="review-disclosure" data-part="review"><summary>人工评审${sample.review ? ` · ${escapeHTML(reviewName(sample.review))}` : ''}</summary>
      <p class="field-note">评审单独留存，不触发自动暂停或解除。</p>
      <div class="sample-review"><label>结论<select aria-label="${escapeHTML(model + ' ' + label + ' ' + annotation)} 人工评审" id="${idPrefix}review-${recordIndex}">
        <option value="unknown" ${review === 'unknown' ? 'selected' : ''}>无法判断</option>
        <option value="correct" ${review === 'correct' ? 'selected' : ''}>正确</option>
        <option value="incorrect" ${review === 'incorrect' ? 'selected' : ''}>错误</option></select></label>
        <label>评审备注<input id="${idPrefix}review-note-${recordIndex}" maxlength="1000" placeholder="记录判断依据" aria-label="评审备注" value="${escapeHTML(sample.review_note || '')}"></label>
        <button class="subtle small" data-review-sample="${recordIndex}" ${isActive ? 'disabled' : ''}>保存评审</button>
      </div>${isActive ? '<p class="field-note">任务结束后可保存人工评审。</p>' : ''}
    </details>`;
  const reason = sample.error || (!simple || sample.reason !== 'manual assessment required' ? sample.reason : '');
  return `<article class="sample${control ? ' control-sample' : ''}" data-key="${control ? 'control' : 'record'}:${recordIndex}" data-model="${escapeHTML(model)}" data-sample="${number === null ? '' : ordinal}">
    <div class="sample-heading" data-part="heading"><span class="sample-model-label" title="${escapeHTML(model)}">${escapeHTML(model)}</span><h4 class="sample-title">${number === null ? '' : `<span class="sample-number" aria-hidden="true">${String(number).padStart(2, '0')}</span>`}<span>${label}</span></h4>${annotation ? tag(annotation, 'warn') : ''}${statusHTML}</div>
    <pre class="answer-text ${isCode ? 'answer-code' : 'answer-prose'}" data-part="output" tabindex="0" aria-label="${escapeHTML(model + ' ' + label + (annotation ? ' ' + annotation : ''))} 回答">${escapeHTML(output || emptyText)}</pre>
    <p class="sample-reason${reason || sample.upstream_condition ? '' : ' hidden'}" data-part="reason">${[sample.upstream_condition && upstreamLabel(sample.upstream_condition), reason && reasonLabel(reason)].filter(Boolean).map(escapeHTML).join(' · ')}</p>
    <div class="sample-footer" data-part="footer"><div class="sample-meta" data-part="meta"><span>总耗时 <strong>${duration(sample.total_ms)}</strong></span><span>首字时间 <strong>${duration(sample.ttft_ms)}</strong></span><span>用量 <strong>${escapeHTML(formatSampleUsage(sample))}</strong></span></div>
    ${actions}</div>${reviewHTML}</article>`;
}

function renderAbsentSample(model, ordinal, simple, isActive, pruned, unlocated) {
  let status = simple ? '回答缺失' : '样本缺失';
  let reason = simple ? '未取得这份回答，请查看执行状态。' : '本轮未取得该样本，不能作为完整失败证据。';
  if (pruned) {
    status = '历史已裁剪';
    reason = '这份回答已按规则的保留上限移除。';
  } else if (unlocated) {
    status = '无留存回答';
    reason = '旧记录未保存全部裁剪位置，无法确认此处是缺答还是已裁剪。';
  } else if (isActive) {
    status = '等待结果';
    reason = '后台按计划执行，结果取得后在此更新。';
  }
  return `<article class="sample sample-pending" data-key="slot:${ordinal}" data-model="${escapeHTML(model)}" data-sample="${ordinal}"><div class="sample-heading" data-part="heading"><span class="sample-model-label" title="${escapeHTML(model)}">${escapeHTML(model)}</span><h4 class="sample-title"><span class="sample-number" aria-hidden="true">${String(ordinal + 1).padStart(2, '0')}</span><span>${simple ? '回答' : '样本'} ${ordinal + 1}</span></h4>${tag(status, isActive || pruned || unlocated ? 'neutral' : 'warn')}</div><p class="empty-line" data-part="pending">${reason}</p></article>`;
}

export function renderRunEvidence(run = {}, credentialName = id => id, { qualityDetail = false } = {}) {
  const config = run.config || {};
  const skip = admissionEvidence(run);
  const credentialLabel = skip?.email || run.credential_email || credentialName(skip?.id || config.auth_id);
  if (skip) return {
    metaHTML: skip.status,
    conditionsHTML: renderFrozenConditions(run, credentialLabel, credentialName),
    samplesHTML: '', completed: 0, planned: 0,
  };
  const samples = run.samples || [];
  const simple = run.source === 'manual_test' && config.judge?.mode === 'manual';
  const isActive = active(run);
  const plannedModels = new Set(config.models?.length ? config.models : samples.map(sampleModel));
  const models = [...new Set([...plannedModels, ...samples.map(sampleModel)])];
  const count = Number.isInteger(config.samples) && config.samples > 0 ? config.samples : 1;
  const planned = plannedModels.size * count;
  const inPlan = sample => plannedModels.has(sampleModel(sample)) && Number.isInteger(sample.index) && sample.index >= 0 && sample.index < count;
  const completed = new Set(samples.filter(s => inPlan(s) && completedSample(s)).map(s => JSON.stringify([sampleModel(s), s.index]))).size;
  const pruned = run.pruned_samples || 0;
  const prunedSlots = run.pruned_sample_slots || [];
  const unlocated = pruned > prunedSlots.length;
  const { correct, incorrect, unknown } = qualityCounts(samples);
  const verdictSummary = [correct && `正确 ${correct}`, incorrect && `答错 ${incorrect}`, unknown && `无法判断 ${unknown}`].filter(Boolean).join(' · ');
  const reference = config.judge?.mode === 'deterministic' && config.question?.answer;
  const metaHTML = simple
    ? `<div class="run-state-line">${statusTag(run.status)}<strong class="run-model" title="${escapeHTML(models.join(' / '))}">${escapeHTML(models.join(' / '))}</strong>${renderManualVerdict(run)}</div><div class="run-context-line"><span>${escapeHTML(questionTitle(config.question, config.question_id))}</span><span>${escapeHTML(date(run.created_at))}</span></div>${run.error || run.block_reason ? `<p class="notice error">${escapeHTML(run.error || run.block_reason)}</p>` : ''}`
    : `<div class="run-state-line">${statusTag(run.status)}<strong>${run.pruned_samples ? `保留 ${samples.length} 份结果` : `${completed} / ${planned} 样本已结束`}</strong><span>${verdictSummary}</span></div>
    <div class="run-context-line"><span>凭证 <strong>${escapeHTML(evidenceValue(credentialLabel))}</strong></span><span>${escapeHTML(questionTitle(config.question, config.question_id))}</span>${reference ? `<span>标准答案 <strong>${escapeHTML(reference)}</strong></span>` : ''}<span>请求强度 ${escapeHTML(config.parameters?.reasoning_effort || '未指定')}</span><span>凭证处理 · ${escapeHTML(credentialHandlingLabel(run))}</span></div>
    ${run.error || run.block_reason ? `<p class="notice error">${escapeHTML(run.error || run.block_reason)}</p>` : ''}`;
  const retained = samples.map((sample, i) => ({ sample, recordIndex: pruned + i }));
  const groups = models.map(model => {
    const entries = retained.filter(({ sample }) => sampleModel(sample) === model);
    const plannedCount = plannedModels.has(model) ? count : 0;
    const slots = Array.from({ length: plannedCount }, (_, ordinal) => entries.filter(({ sample }) => sample.index === ordinal));
    const content = slots.map((records, ordinal) => {
      const removed = prunedSlots.some(s => s.model === model && s.index === ordinal);
      if (!records.length) return renderAbsentSample(model, ordinal, simple, isActive, removed, unlocated);
      return records.map(({ sample, recordIndex }) => renderAnswer(sample, recordIndex, ordinal, isActive, {
        simple, qualityDetail, annotation: records.length > 1 || removed ? `序号重复 · 原记录 ${recordIndex + 1}` : '',
      })).join('');
    }).join('');
    const extras = entries.filter(({ sample }) => !inPlan(sample)).map(({ sample, recordIndex }) => renderAnswer(sample, recordIndex, sample.index, isActive, {
      simple, qualityDetail, annotation: `计划外记录 · 原记录 ${recordIndex + 1}`,
    })).join('');
    const occupied = slots.filter(records => records.length).length;
    const summary = plannedCount ? `${occupied} / ${plannedCount} 份结果${occupied === entries.length ? '' : ` · 保留 ${entries.length} 条记录`}` : `计划外模型 · 保留 ${entries.length} 条记录`;
    return `<section class="model-result" data-key="${escapeHTML(`model:${model}`)}" data-model="${escapeHTML(model)}"><div class="model-heading" data-part="model-heading"><div class="model-identity"><h3 title="${escapeHTML(model)}">${escapeHTML(model)}</h3><span>${summary}</span></div><button type="button" class="text-button model-copy" data-copy-model="${escapeHTML(model)}" aria-label="复制模型 ID ${escapeHTML(model)}">复制 ID</button></div>${content}${extras}</section>`;
  }).join('');
  const controlHTML = run.controls?.length ? `<details class="control-results" data-key="controls"><summary>同批对照 · ${run.controls.length} 份结果</summary><p class="field-note">只有身份不同、同题、同模型映射、同有效条件的通过样本才构成有效对照。</p><div class="comparison-grid">${run.controls.map((sample, index) => `<section class="model-result" data-key="${escapeHTML(`control-model:${index}:${sample.model || '模型未知'}`)}"><div class="model-heading" data-part="model-heading"><h3>${escapeHTML(sample.model || '模型未知')}</h3><span>对照凭证</span></div>${renderAnswer(sample, index, sample.index ?? index, isActive, { control: true, qualityDetail })}</section>`).join('')}</div></details>` : '';
  return {
    metaHTML,
    conditionsHTML: renderFrozenConditions(run, credentialLabel, credentialName),
    samplesHTML: `<div class="comparison-grid${simple ? ` question-test-results${models.length === 1 ? ' single-question' : ''}` : ''}" data-key="models">${groups || '<p class="empty-line">本轮没有记录模型或样本。</p>'}</div>${controlHTML}`,
    completed,
    planned,
  };
}

export function renderQualityDetail(run, rules, credentialName) {
  const config = run.config || {};
  const ruleID = run.rule_id || config.rule_id;
  const ruleName = rules.find(rule => rule.id === ruleID)?.name || ruleID || '规则检测';
  const { conditionsHTML, samplesHTML, completed, planned } = renderRunEvidence(run, credentialName, { qualityDetail: true });
  const samples = (run.samples || []).filter(completedSample);
  const { correct, incorrect, unknown } = qualityCounts(samples);
  const verdict = [[correct, '正确', 'good'], [incorrect, '错误', 'warn'], [unknown, '无法判断', 'neutral']]
    .filter(([count]) => count).map(([count, label, tone]) => tag(planned === 1 && samples.length === 1 ? label : `${label} ${count}`, tone)).join('');
  const reference = config.judge?.mode === 'deterministic' ? config.question?.answer : config.review_standard || config.question?.answer;
  const issue = run.error || run.block_reason || run.reason;
  const skip = admissionEvidence(run);
  const label = skip?.email || run.credential_email || credentialName(skip?.id || config.auth_id) || '凭证未记录';
  const conditions = `<details class="quality-detail-conditions"><summary>检测配置与原始记录</summary><div class="run-identifier">任务 ${escapeHTML(run.id || run.run_id)}</div>${conditionsHTML}</details>`;
  const header = `<header class="quality-detail-identity"><strong>${escapeHTML(label)}</strong>${skip?.id && skip.id !== label ? `<code>${escapeHTML(skip.id)}</code>` : ''}
    <div><span>${escapeHTML(ruleName)}</span><time datetime="${escapeHTML(run.created_at || '')}">${escapeHTML(date(run.created_at))}</time></div></header>`;
  if (skip) return header + `<dl class="quality-detail-facts">
    <div><dt>运行状态</dt><dd>${skip.status}<small>${escapeHTML(skip.explanation)}</small></dd></div>
    ${evidenceFact('判断依据', skip.source)}${skip.model ? evidenceFact('涉及模型', skip.model) : ''}
    ${evidenceFact(skip.reasonLabel, skip.reason)}${evidenceFact('凭证处理', credentialHandlingLabel(run))}
    ${run.credential_action && credentialActions[run.credential_action] ? evidenceFact('处置动作', credentialActions[run.credential_action]) : ''}
  </dl>${issue && reasonLabel(issue) !== skip.reason ? `<p class="notice error">${escapeHTML(reasonLabel(issue))}</p>` : ''}${conditions}`;
  return header + `
    <dl class="quality-detail-facts">
      <div><dt>答案判定</dt><dd>${verdict || (active(run) ? '等待判定' : '暂无判定')}</dd></div>
      <div><dt>运行状态</dt><dd>${statusTag(run.status)}<small>${run.pruned_samples ? `保留 ${samples.length} 条已结束记录 · 已裁剪 ${escapeHTML(run.pruned_samples)} 条` : `${completed} / ${planned} 份样本`}</small></dd></div>
      ${run.upstream_condition ? evidenceFact('上游错误', upstreamLabel(run.upstream_condition)) : ''}${evidenceFact('凭证处理', credentialHandlingLabel(run))}
    </dl>
    ${issue ? `<p class="notice error">${escapeHTML(reasonLabel(issue))}</p>` : ''}
    <div class="quality-detail-question"><strong>${escapeHTML(questionTitle(config.question, config.question_id))}</strong>
      ${reference == null || reference === '' ? '' : `<div><span>${config.judge?.mode === 'deterministic' ? '标准答案' : '参考标准'}</span><strong>${escapeHTML(reference)}</strong></div>`}</div>
    <section class="quality-detail-answers" aria-label="模型回答">${samplesHTML}</section>
    ${conditions}`;
}
// Pure operation views keep restriction ownership separate from rule state.
const restrictionLabels = {
  manual_disabled: '人工禁用',
  operator_disabled: '人工禁用',
  quality_pause: '质量暂停',
  quality_paused: '质量暂停',
  native_cooldown: '原生冷却',
  cooldown: '原生冷却',
  unavailable: '当前不可用',
  native_unavailable: '宿主标记暂不可用',
  runtime_status_unavailable: '运行状态尚未确认',
  retained_pause_unverified: '历史暂停归属待核对',
};
const restrictionText = (reason) => restrictionLabels[reason] || String(reason ?? '未知限制');
const officialQualityUnavailable = (target) => target?.unsupported_reason === 'official_host_quality_pause_unavailable';
const operationFact = (label, value) =>
  `<div><dt>${escapeHTML(label)}</dt><dd>${escapeHTML(value == null || value === '' ? '未知' : value)}</dd></div>`;
const restrictionList = (reasons, fallback) =>
  Array.isArray(reasons) && reasons.length
    ? `<ul class="restriction-list">${reasons.map((reason) => `<li>${escapeHTML(restrictionText(reason))}</li>`).join('')}</ul>`
    : `<p class="field-note">${escapeHTML(fallback)}</p>`;

function nextRuleTime(rule) {
  // Keep the offset of this occurrence, including DST, independent of the browser zone.
  const stamp = rule.next_run_local || rule.next_run || rule.next_run_at || '';
  const match = /^(\d{4}-\d\d-\d\d)T(\d\d:\d\d:\d\d)(?:\.\d+)?(Z|[+-]\d\d:\d\d)$/.exec(stamp);
  if (!match || stamp.startsWith('0001-')) return '暂无下次执行时间';
  return `下次 ${match[1]} ${match[2]} · UTC${match[3] === 'Z' ? '' : match[3]}`;
}

export function renderQualityEmpty(title, description = '', action = '') {
  return `<div class="quality-empty"><svg class="control-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M5 3h14v18H5zM8 7h8M8 11h8M8 15h4"/></svg><strong>${escapeHTML(title)}</strong>${description ? `<span>${escapeHTML(description)}</span>` : ''}${action}</div>`;
}

export function ruleState(rule, targets) {
  if (holdsPause(rule, targets) || rule.enabled !== true || typeof rule.timer_active !== 'boolean') return 'stopped';
  return rule.timer_active ? 'timed' : 'manual';
}

export function dynamicScopeLabel(scope) {
  return scope === 'all_credentials' ? '全部凭证' : scope === 'all_enabled' ? '全部已启用凭证' : '';
}

function handlingRecord(record, name) {
  const upstream = record.upstream_condition;
  const catalogModel = record.catalog_missing_model;
  const resolved = record.state === 'disabled' || record.state === 'deleted';
  const evidence = catalogModel ? `CPA 注册目录缺少 ${catalogModel}` : upstream && upstreamLabel(upstream);
  const status = record.state === 'disabled' ? (evidence ? '已按错误规则暂停，退出后续检测' : '已暂停凭证') : record.state === 'deleted' ? '已移除凭证' : '处置待核对';
  const confirmedCondition = evidence && resolved;
  const summary = [record.email || name(record.auth_id), evidence, status].filter(Boolean).map(escapeHTML).join(' · ');
  const check = `<button type="button" class="text-button" data-handling-ack="${escapeHTML(record.action_id)}">核对人工处理</button>`;
  const followup = confirmedCondition && record.state === 'deleted' ? '。后续检测无需人工标记。' : '';
  return `<div><span>${summary}${followup}</span>${confirmedCondition ? `<details><summary>已在 CPA 手动变更</summary>${check}</details>` : check}</div>`;
}

export function renderScopeSkipDetails(dispatch) {
  const rows = Array.isArray(dispatch.details) ? dispatch.details.filter(row => row && typeof row === 'object' && !Array.isArray(row)) : [];
  if (!rows.length) {
    const disabled = dispatch.skipReasons?.manual_disabled;
    const aggregate = Number.isSafeInteger(disabled) && disabled > 0 ? `其中人工禁用 ${disabled} 个。` : '';
    return `<p class="field-note">${aggregate}本轮未记录逐凭证明细。</p>`;
  }
  const note = rows.length < dispatch.skipped ? `<p class="field-note">已记录其中 ${rows.length} 个凭证的明细。</p>` : '';
  return `${note}<ul class="scope-skip-list" tabindex="0" aria-label="跳过的凭证和原因">${rows.map(row => {
    const identity = row.email || row.auth_id || '凭证标识未记录';
    const id = row.email && row.auth_id && row.email !== row.auth_id ? `<code>${escapeHTML(row.auth_id)}</code>` : '';
    const link = row.run_id ? `<button type="button" class="text-button small" data-run="${escapeHTML(row.run_id)}" aria-label="查看 ${escapeHTML(row.auth_id || identity)} 的跳过详情">详情</button>` : '';
    return `<li${row.auth_id ? ` data-key="skip:${escapeHTML(row.auth_id)}"` : ''}><div><strong>${escapeHTML(identity)}</strong>${id}${link}</div><p>${row.model ? `<code>${escapeHTML(row.model)}</code> · ` : ''}${escapeHTML(row.reason || row.code || '原因未记录')}</p></li>`;
  }).join('')}</ul>`;
}

export function renderScopeDispatch(dispatch, title, ruleID, expanded = false) {
  const count = value => Number.isSafeInteger(value) && value >= 0 ? value : '未知';
  const at = dispatch.at ? ` <time datetime="${escapeHTML(dispatch.at)}">${escapeHTML(date(dispatch.at))}</time>` : '';
  const header = `<span class="dispatch-heading">${escapeHTML(title)}${at}</span><span class="dispatch-counts">纳入检测 ${count(dispatch.matched)} 个 · 跳过 ${count(dispatch.skipped)} 个</span>`;
  if (!(dispatch.skipped > 0 || dispatch.details?.length)) return `<div class="scope-dispatch scope-dispatch-summary">${header}</div>`;
  return `<details class="scope-dispatch" data-part="dispatch-skips" data-skip-rule="${escapeHTML(ruleID)}"><summary>${header}</summary><div class="scope-skip-body">${expanded ? renderScopeSkipDetails(dispatch) : ''}</div></details>`;
}

export function renderRuleRows(rules, targets, { connected, targetsFresh, settings = {}, scopeRule = '', busy, resourceStatus, expandedSkips }) {
  const emails = new Map(targets.map(t => [targetID(t), t.email]));
  const name = id => emails.get(id) || id;
  if (!rules.length) return renderQualityEmpty('还没有检测规则', '创建一条规则，按需手动执行或开启定时检测。', '<button class="subtle" data-rule-new>创建第一条规则</button>');
  const officialHost = settings.capabilities?.mode === 'official_observation' || targets.some(officialQualityUnavailable);
  return rules.map((rule) => {
    const config = rule.config || rule, held = holdsPause(rule, targets);
    const rulesCurrent = resourceStatus?.rules !== 'error';
    const ready = connected && targetsFresh && rulesCurrent;
    const enabled = rule.enabled === true && !held, timed = rule.timer_active;
    const known = typeof rule.enabled === 'boolean' && typeof timed === 'boolean';
    const unsupported = rule.status === 'unsupported' || (officialHost && (rule.action === 'pause_credential' || rule.automatic_pause));
    const on = enabled && timed === true;
    const saving = busy?.has(`rule-timer:${rule.id}`);
    const label = held ? officialHost ? '历史归属待核对' : '等待人工解除' : unsupported ? '不支持' : !known ? '状态未确认' : on ? '运行中' : '已暂停';
    const canSwitch = officialHost && known && ready && !held && !unsupported;
    const hint = saving ? '正在保存' : canSwitch ? (on ? '暂停定时检测' : '启用定时检测') : label;
    const blockedBy = held ? (rule.blocked_auth_id ? `受暂停凭证 ${name(rule.blocked_auth_id)} 影响` : '目标或依赖凭证暂停，所有检测停止') : !rulesCurrent ? '规则状态未确认，请刷新' : !ready ? '凭证状态未确认，请刷新' : '';
    const id = escapeHTML(rule.id), selected = scopeRule === rule.id;
    const schedule = !enabled ? '已停止检测' : timed === true ? (rule.cron || `每 ${rule.interval_seconds ?? '未知'} 秒`) : timed === false ? '按需手动执行' : '定时状态未确认';
    const actionSummary = rule.action === 'disable_credential' ? `暂停凭证${rule.auto_restore ? '；整轮通过后自动恢复' : '；手动恢复'}` : rule.action === 'delete_credential' ? '从 CPA 移除凭证' : rule.action === 'observe_only' ? '仅记录' : rule.action === 'pause_credential' ? officialHost ? '历史暂停配置；官方宿主不执行' : '凭证暂停配置' : '处置未知';
    const upstreamFacts = configuredUpstreamActions(rule.upstream_error_actions).map(([label, action]) => operationFact(label, action)).join('');
    return `<article class="quality-rule${selected ? ' selected' : ''}" data-key="rule:${id}">
      <div class="quality-rule-head"><button class="quality-rule-select" data-rule-select="${id}" aria-pressed="${selected}" title="查看此规则的运行记录">${escapeHTML(rule.name || rule.id)}</button><button type="button" class="rule-state rule-timer-toggle" role="switch" aria-label="${escapeHTML(rule.name || rule.id)}的定时检测" aria-checked="${on}" aria-busy="${!!saving}" data-rule-timer="${id}" title="${hint}" ${!canSwitch || saving ? 'disabled' : ''}>${label}</button></div>
      <p class="quality-rule-scope"><svg class="control-icon" viewBox="0 0 20 20" aria-hidden="true"><circle cx="10" cy="7" r="3"/><path d="M4 17v-1a6 6 0 0 1 12 0v1"/></svg><span>${dynamicScopeLabel(rule.target_scope) ? dynamicScopeLabel(rule.target_scope) + ' · 动态纳入' : escapeHTML(name(config.auth_id) || '目标未知')}</span></p>
      ${rule.last_scope_dispatch ? renderScopeDispatch(rule.last_scope_dispatch, '最近调度', rule.id, expandedSkips?.has(rule.id))
        : rule.target_scope === 'all_credentials' && rule.last_manual_skips?.length ? `<details><summary>上轮跳过 ${rule.last_manual_skips.length} 个手动禁用凭证</summary><ul class="restriction-list">${rule.last_manual_skips.map(id => `<li>${escapeHTML(name(id))}${name(id) !== id ? ` · ${escapeHTML(id)}` : ''} · 人工禁用</li>`).join('')}</ul></details>` : ''}
      <div class="quality-rule-models">${(config.models || []).map(model => `<code>${escapeHTML(model)}</code>`).join('') || '<small>模型未记录</small>'}</div>
      <details><summary>${escapeHTML(config.models?.length ?? 0)} 个模型 · 请求强度 ${escapeHTML(config.parameters?.reasoning_effort || '未指定')}</summary><dl class="evidence-facts">${operationFact('题目', questionTitle(config.question, config.question_id))}${operationFact('每模型样本', config.samples)}${operationFact('规则版本', rule.revision)}${operationFact('对照凭证', name(config.control_auth_id) || '未配置')}${operationFact('判题', config.judge?.mode === 'llm' ? `独立模型 · ${name(config.judge.auth_id) || '未知'} · ${config.judge.model || '未知'}` : config.judge?.mode === 'deterministic' ? '标准答案判定' : config.judge?.mode === 'manual' ? '人工评审' : '未知')}${operationFact('答错后的处理', actionSummary)}${upstreamFacts}${rule.action === 'pause_credential' ? operationFact('历史自动暂停配置', rule.automatic_pause === true ? '开；当前执行能力另行核验' : rule.automatic_pause === false ? '关' : '未知') : ''}</dl></details>
      ${rule.handling?.length ? `<div class="quality-handling-records">${rule.handling.map(record => handlingRecord(record, name)).join('')}</div>` : ''}
      <div class="quality-rule-plan"><span>${enabled && timed ? '下次检测' : '运行方式'}</span><strong>${enabled && timed ? `${escapeHTML(nextRuleTime(rule))}<small>${escapeHTML(schedule)}</small>` : escapeHTML(schedule)}</strong></div>
      ${blockedBy ? `<p class="quality-rule-warning">${escapeHTML(blockedBy)}</p>` : ''}
      <footer class="quality-rule-actions"><button class="text-button" data-rule-edit="${id}">编辑</button><button class="text-button" data-rule-delete="${id}" ${held || !ready ? 'disabled' : ''}>删除</button>${rule.last_scope_batch ? `<button class="text-button" data-rule-batch="${escapeHTML(rule.last_scope_batch)}">最近批次</button>` : rule.last_run ? `<button class="text-button" data-run="${escapeHTML(rule.last_run)}">最近结果</button>` : ''}<button data-rule-run="${id}" ${!ready || !enabled ? 'disabled' : ''}>立即执行</button></footer>
    </article>`;
  }).join('');
}

export function renderQualityRuns(runs, rules, credentialName = id => id) {
  if (!runs.length) return renderQualityEmpty('暂无运行记录', '手动或定时执行规则后，在这里查看检测结果。');
  return `<table class="quality-run-table"><thead><tr><th scope="col">检测时间</th><th scope="col">凭证 / 规则</th><th scope="col">检测结果</th><th scope="col">凭证处理</th><th scope="col"><span class="sr-only">操作</span></th></tr></thead><tbody>${runs.map(run => {
    const ruleID = run.rule_id || run.config?.rule_id;
    const skip = admissionEvidence(run);
    const samples = run.samples || [], { correct, incorrect, unknown } = qualityCounts(samples);
    const result = skip ? `<span class="muted">${escapeHTML(skip.result)}</span>` : samples.length ? `<span class="good">正确 ${correct}</span><span${incorrect ? ' class="bad"' : ''}>答错 ${incorrect}</span><span class="muted">未知 ${unknown}</span>${run.pruned_samples ? `<span class="muted">已裁剪 ${escapeHTML(run.pruned_samples)}</span>` : ''}` : '<span class="muted">暂无判定</span>';
    const ruleName = rules.find(r => r.id === ruleID)?.name || ruleID || '规则检测';
    const stamp = date(run.created_at), [day, clock] = stamp.split(' ');
    const handling = credentialHandlingLabel(run);
    const identity = skip?.email || run.credential_email || credentialName(skip?.id || run.config?.auth_id) || '凭证未记录';
    const issue = skip?.reason || run.error || run.reason || run.block_reason;
    const handlingTone = run.action_status === 'credential_restored' ? 'good' : run.action_status === 'credential_removed' ? 'bad' : ['credential_suspended', 'credential_action_uncertain', 'credential_action_attention', 'management_session_required', 'pending_reconciliation', 'awaiting_release'].includes(run.action_status) ? 'warn' : 'neutral';
    return `<tr data-key="run:${escapeHTML(run.id || run.run_id)}">
      <td class="quality-time-cell" data-label="检测时间"><time datetime="${escapeHTML(run.created_at || '')}" title="${escapeHTML(stamp)}"><strong>${escapeHTML(clock || day)}</strong>${clock ? `<small>${escapeHTML(day)}</small>` : ''}</time></td>
      <td class="quality-identity-cell" data-label="凭证 / 规则"><strong>${escapeHTML(identity)}</strong>${skip?.id && skip.id !== identity ? `<small>${escapeHTML(skip.id)}</small>` : ''}<small>${escapeHTML(ruleName)}</small><small class="quality-run-parameters"><span>${escapeHTML(run.config?.models?.join(' / ') || '模型未记录')}</span>${skip ? '' : `<span>请求强度 ${escapeHTML(run.config?.parameters?.reasoning_effort || '未指定')}</span>`}</small></td>
      <td class="quality-result-cell" data-label="检测结果">${skip?.status || statusTag(run.status)}<div class="quality-result">${result}</div>${issue ? `<small class="${skip ? 'muted' : 'error-note'}">${skip?.model ? escapeHTML(skip.model) + ' · ' : ''}${escapeHTML(issue)}</small>` : ''}</td>
      <td class="quality-handling-cell" data-label="凭证处理">${tag(handling, handlingTone)}${skip ? `<small>${escapeHTML(skip.source)}</small>` : run.upstream_condition ? `<small>${escapeHTML(upstreamLabel(run.upstream_condition))}</small>` : ''}</td>
      <td class="quality-detail-cell"><button class="text-button" data-run="${escapeHTML(run.id || run.run_id)}" aria-label="查看${escapeHTML(ruleName)}的运行详情">详情 <span aria-hidden="true">→</span></button></td>
    </tr>`;
  }).join('')}</tbody></table>`;
}

export function renderTargetRows(targets) {
  if (!targets.length) return '<tr><td colspan="3" class="empty-line"><div class="calm-empty"><strong>没有可展示的凭证</strong><span>请连接并刷新，或先在 CPA 添加凭证。列表缺项不表示原有暂停已解除。</span></div></td></tr>';
  return targets.map((target) => {
    const reasons = target.reasons ?? target.unavailable_reasons;
    const officialHost = officialQualityUnavailable(target);
    const flags = [
      target.manual_disabled === true ? tag(target.credential_handling === 'disabled' ? '规则暂停' : '人工禁用', 'bad') : target.manual_disabled === false ? '' : tag('人工禁用状态未知'),
      target.quality_paused === true ? tag('质量暂停 · 整个凭证', 'warn') : target.quality_paused === false ? '' : tag('质量暂停状态未知'),
    ].filter(Boolean);
    const clearStates = [
      target.manual_disabled === false ? '未人工禁用' : '',
      target.quality_paused === false ? '无质量暂停' : '',
    ].filter(Boolean);
    const otherReasons = Array.isArray(reasons) ? reasons.filter((reason) =>
      !(target.manual_disabled === true && ['manual_disabled', 'operator_disabled'].includes(reason)) &&
      !(target.quality_paused === true && ['quality_pause', 'quality_paused'].includes(reason))) : null;
    const identity = targetID(target);
    return `<tr data-key="target:${escapeHTML(targetID(target))}">
      <td data-label="凭证"><div class="table-identity"><strong>${escapeHTML(target.email || target.auth_index || identity || '未知')}</strong>${target.auth_index && target.auth_index !== identity || !identity ? `<small class="table-id">${escapeHTML(identity || '标识未知')}</small>` : ''}<span class="target-provider">${escapeHTML(target.provider || '未知')}<span> · ${escapeHTML(({config_api_key:'配置密钥',auth_file:'认证文件',oauth:'OAuth'})[target.type] || target.type || '类型未知')}</span></span></div></td>
      <td data-label="可用状态">${tag(target.effective_available === true ? '可用' : target.effective_available === false ? '不可用' : '状态未知', target.effective_available === true ? 'good' : 'neutral')}</td>
      <td data-label="限制说明"><div class="target-restrictions">${flags.length ? `<div class="button-row target-restriction-flags">${flags.join('')}</div>` : ''}${clearStates.length ? `<small>${escapeHTML(clearStates.join(' · '))}</small>` : ''}${officialHost && target.quality_paused === true ? '<small>历史暂停归属仍保留</small>' : ''}${otherReasons === null ? '<p class="field-note">其他限制原因未知</p>' : otherReasons.length ? restrictionList(otherReasons, '') : ''}${!officialHost && target.unsupported_reason ? `<small>${escapeHTML(target.unsupported_reason)}</small>` : ''}</div></td>
    </tr>`;
  }).join('');
}

export function renderActionItems(actions, targets) {
  if (!actions.length) return '<div class="empty-line calm-empty"><strong>没有保留的质量暂停归属</strong><span>当前仅检测与观察。凭证是否可用，请查看凭证状态。</span></div>';
  const officialHost = targets.some(officialQualityUnavailable);
  return actions.map((action, index) => {
    const target = targets.find((entry) => targetID(entry) === action.auth_id);
    const owned = action.owner_plugin === 'model-quality';
    const retained = action.status === 'retained_unverified';
    // Target capability only admits a preview; the host still validates release authority.
    const capabilityConfirmed = !officialHost && target?.can_pause === true;
    const canPreview = capabilityConfirmed && owned && !!(action.action_id || action.id) && action.action_revision != null;
    const capabilityNote = officialHost ? '官方接口不支持创建或解除质量暂停。此历史归属记录继续保留。'
      : !target ? '凭证当前不可发现，解除能力尚未确认。此暂停归属记录继续保留。'
      : !capabilityConfirmed ? '宿主未确认独立质量暂停能力，暂不可解除。此暂停归属记录继续保留。' : '';
    return `<article class="action-item" data-key="action:${escapeHTML(action.action_id || action.id || action.auth_id)}">
      <div class="action-summary"><div class="button-row">${tag(retained ? '历史归属待核对' : '质量暂停', 'warn')}<strong>${escapeHTML(target?.email || target?.auth_index || action.auth_id || '凭证未知')}</strong></div><p>${escapeHTML(action.reason ? reasonLabel(action.reason) : '暂停原因未提供')}</p><small>${retained ? '归属记录已保留，当前宿主未验证暂停状态' : '影响整个凭证所有模型的新业务执行'} · ${escapeHTML(date(action.created_at || action.time))}</small><dl class="evidence-facts">${operationFact('暂停归属', action.owner_plugin)}${operationFact('检测规则', action.rule_id)}${operationFact('动作版本', action.action_revision)}</dl></div>
      <div class="action-decision">${action.evidence_run_id ? `<button class="text-button" data-run="${escapeHTML(action.evidence_run_id)}">查看失败证据</button>` : ''}${capabilityNote ? `<p>${escapeHTML(capabilityNote)}</p>` : ''}<p>${target?.manual_disabled ? '同时存在人工禁用；解除质量暂停后仍然生效。' : target ? '解除后相关检测规则仍停止，须另行恢复。' : '凭证当前不可发现；解除前需核对身份与剩余限制。'}</p><button class="danger small" data-action-preview="${index}" ${canPreview ? '' : 'disabled'}>${capabilityConfirmed ? '预览并解除自身暂停' : '解除质量暂停不可用'}</button>${!owned ? '<small>仅可解除本插件持有的暂停</small>' : ''}</div>
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
    <section class="release-scope"><h3>${escapeHTML(target?.email || target?.auth_index || action.auth_id || '影响凭证未知')}</h3><p>仅释放下面这一条归属匹配的质量暂停，作用范围为整个凭证的所有模型。</p><dl class="evidence-facts">${operationFact('凭证标识', action.auth_id)}${operationFact('暂停归属', action.owner_plugin)}${operationFact('关联规则', action.rule_id)}${operationFact('动作标识', action.action_id || action.id)}${operationFact('动作版本', action.action_revision)}${operationFact('凭证身份版本', action.identity_version)}</dl></section>
    <section class="release-scope"><h3>解除后仍需处理</h3>${restrictionList(knownRemaining, hasRemaining ? '预览未报告其他限制；最终可用状态以解除后的宿主查询为准。' : '剩余限制尚不能完整确定，解除后须重新读取宿主状态。')}<p class="field-note">人工禁用、原生冷却与其他归属的暂停继续生效。当前质量暂停标记不能证明只有一个暂停归属。</p></section>
    <details><summary>原始解除预览 JSON</summary><pre>${escapeHTML(JSON.stringify(preview, null, 2))}</pre></details>`;
}
