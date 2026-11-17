import { ManagementClient, isDiscarded } from './client.js';
import { createLoginWorkbench } from './login.js';
import {
  active,
  copyRunConfig,
  credentialBlock,
  date,
  escapeHTML,
  holdsPause,
  items,
  parseModels,
  safePreview,
  statusLabels,
  statusTag,
  tag,
  targetID,
  renderRunEvidence,
  renderSampleEvidence,
  renderRuleRows,
  renderTargetRows,
  renderActionItems,
  renderReleasePreview,
  sourceLabel,
  questionTestSelection,
} from './view.js';

const $ = (id) => document.getElementById(id);
const state = {
  connected: false,
  targetsFresh: false,
  targets: [],
  questions: [],
  rules: [],
  actions: [],
  runs: [],
  run: null,
  historyOffset: 0,
  historyTotal: 0,
  poll: null,
  pollFailures: 0,
  release: null,
  settings: {},
  page: 'manual',
  manualView: 'results',
  readingExpanded: false,
  opsView: 'attention',
  releaseBlocked: true,
  releaseAction: null,
  compareRun: null,
  reads: { run: 0, history: 0, operations: 0, refresh: 0, preview: 0, editor: 0 },
  busy: new Set(),
  editor: null,
  resourceStatus: {},
};
const value = (id) => $(id).value.trim();
const number = (id) => Number($(id).value);
const checked = (id) => $(id).checked;
const findTarget = (id) => state.targets.find((target) => targetID(target) === id);
const ruleHeld = (rule) => holdsPause(rule, state.targets);
const managementPrefix = '/v0/management/plugins/model-quality';
const managementKeyStorage = `model-quality:management-key:${managementPrefix}`;
const client = new ManagementClient(managementPrefix, (key) =>
  disconnect('管理鉴权已失效，请重新连接。', true, { expectedKey: key }),
);
const api = (path, method = 'GET', body) => client.request(path, method, body);
const loginWorkbench = createLoginWorkbench({
  request: (path, method = 'GET', body, idempotencyKey) => client.requestHostLogin(path, method, body, idempotencyKey),
  connected: () => state.connected,
  onCommitted: () => refresh(),
});

let stopThemeFollowing = () => {};
let localTheme = ['light', 'white', 'dark'].includes($('theme-toggle').value) ? $('theme-toggle').value : 'light';
function initializeTheme() {
  stopThemeFollowing();
  stopThemeFollowing = () => {};
  const selector = $('theme-toggle');
  const apply = (theme, followsHost = false) => {
    document.documentElement.dataset.theme = theme;
    document.documentElement.dataset.themeSource = followsHost ? 'host' : 'local';
    document.body.classList.toggle('host-themed', followsHost);
    selector.hidden = followsHost;
    selector.disabled = followsHost;
    selector.value = theme;
  };
  apply(localTheme);
  try {
    if (window.parent === window) return;
    const parentDocument = window.parent.document;
    // Recognize the pinned management frontend without accessing authentication or storage.
    if (parentDocument.title.trim() !== 'CLI Proxy API Management Center') return;
    const parentRoot = parentDocument.documentElement;
    const update = () => {
      try {
        const applied = parentRoot.getAttribute('data-theme');
        if (applied === null) apply('light', true);
        else if (applied === 'white' || applied === 'dark') apply(applied, true);
        else apply(localTheme);
      } catch {
        apply(localTheme);
      }
    };
    const observer = new MutationObserver(update);
    observer.observe(parentRoot, { attributes: true, attributeFilter: ['data-theme'] });
    stopThemeFollowing = () => observer.disconnect();
    update();
  } catch {
    // Cross-origin and unrecognized parents keep the independent page controls.
    apply(localTheme);
  }
}

let noticeTimer;
const feedbackScope = () => `${state.page}:${state.page === 'manual' ? state.manualView : state.opsView}`;
function clearNotice() {
  clearTimeout(noticeTimer);
  $('notice').classList.add('hidden');
  $('notice').textContent = '';
}

function notice(message, error = false) {
  clearNotice();
  $('notice').textContent = message;
  $('notice').classList.remove('hidden');
  $('notice').classList.toggle('error', error);
  const dialog = document.querySelector('dialog[open], .inline-editor:not(.hidden)');
  if (error && dialog && dialog.id !== 'release-dialog') {
    let feedback = dialog.querySelector('.dialog-message');
    if (!feedback) {
      feedback = document.createElement('div');
      feedback.className = 'notice error dialog-message';
      feedback.setAttribute('role', 'alert');
      dialog.querySelector('.dialog-heading').after(feedback);
    }
    feedback.textContent = message;
    feedback.tabIndex = -1;
    if (!dialog.querySelector('[aria-invalid="true"]')) feedback.focus({ preventScroll: false });
  }
  if (!error) noticeTimer = setTimeout(clearNotice, 6000);
}

function localFeedback(button, message) {
  const original = button.textContent;
  button.style.minWidth = `${button.getBoundingClientRect().width}px`;
  button.textContent = message;
  $('local-feedback').textContent = message;
  setTimeout(() => {
    if (button.isConnected && button.textContent === message) { button.textContent = original; button.style.removeProperty('min-width'); }
  }, 2500);
}

let actionRequest = null;
function actionDialog({ title, description, confirm = '确认', input = false, danger = false, submit }) {
  if ($('action-dialog').open) return;
  $('action-title').textContent = title;
  $('action-description').textContent = description;
  $('action-name-field').hidden = !input;
  $('action-name').required = input;
  $('action-name').value = '';
  $('action-error').classList.add('hidden');
  $('action-name').removeAttribute('aria-invalid');
  $('action-confirm').textContent = confirm;
  $('action-confirm').className = danger ? 'danger' : 'primary';
  actionRequest = { submit, input, epoch: client.epoch };
  $('action-dialog').showModal();
  $(input ? 'action-name' : 'action-cancel').focus();
}

function editorSnapshot(id) {
  return JSON.stringify([...$(id).querySelectorAll('input,select,textarea')].map((field) => [field.id, field.type === 'checkbox' ? field.checked : field.value]));
}
function editorDirty() { return state.editor && state.editor.snapshot !== editorSnapshot(state.editor.id); }
function updateEditorDirty() {
  if (!state.editor) return;
  $(state.editor.id).querySelector('.editor-dirty').textContent = editorDirty() ? '有未保存更改' : '尚未更改';
}
function protectEditor(action) {
  if (state.editor?.saving) { notice('正在保存当前编辑，请等待结果。'); return; }
  if (!editorDirty()) { action(); return; }
  actionDialog({ title: '放弃未保存的更改？', description: '当前编辑尚未保存。放弃后将关闭这些更改，已保存的规则和预算不受影响。', confirm: '放弃更改', danger: true, submit: action });
}

async function saveEditor(action) {
  const editor = state.editor;
  if (!editor || editor.saving) return;
  editor.saving = true;
  const controls = [...$(editor.id).querySelectorAll('input,select,textarea,button')].map((node) => [node, node.disabled]);
  controls.forEach(([node]) => { node.disabled = true; });
  try { await action(editor); }
  finally {
    editor.saving = false;
    controls.forEach(([node, disabled]) => { node.disabled = disabled; });
    if (state.editor === editor) $(editor.id).querySelector('[aria-invalid="true"]')?.focus();
    renderAdmission();
  }
}

function fieldError(id, message) {
  const field = $(id);
  const errorID = `${id}-error`;
  let feedback = $(errorID);
  if (!feedback) {
    feedback = document.createElement('span');
    feedback.id = errorID;
    feedback.className = 'field-error';
    feedback.setAttribute('role', 'alert');
    field.after(feedback);
  }
  feedback.textContent = message;
  field.setAttribute('aria-invalid', 'true');
  field.setAttribute('aria-describedby', [field.getAttribute('aria-describedby'), errorID].filter(Boolean).join(' '));
  field.closest('details')?.setAttribute('open', '');
  field.focus();
  throw new Error(message);
}

async function safely(action, key = '', button = null) {
  if (key && state.busy.has(key)) return;
  if (key) state.busy.add(key);
  const scope = feedbackScope();
  if (key) document.querySelectorAll('.dialog-message').forEach((node) => node.remove());
  const originalContent = button ? [...button.childNodes] : [];
  const busyLabel = key.includes('save') || key === 'save-rule' ? '保存中…' : key.includes('refresh') ? '读取状态中…' : key === 'connect' ? '连接中…' : key === 'submit-run' ? '提交中…' : '';
  if (button) {
    button.style.minWidth = `${button.getBoundingClientRect().width}px`;
    button.disabled = true;
    button.setAttribute('aria-busy', 'true');
    if (busyLabel) button.textContent = busyLabel;
  }
  try {
    return await action();
  } catch (error) {
    if (!isDiscarded(error) && scope === feedbackScope()) notice(error.message || '操作失败，请重试。', true);
  } finally {
    if (key) state.busy.delete(key);
    if (button) {
      button.removeAttribute('aria-busy');
      button.disabled = false;
      button.style.removeProperty('min-width');
      if (busyLabel && button.textContent === busyLabel) button.replaceChildren(...originalContent);
    }
    renderAdmission();
  }
}

function markFresh() {
  $('data-state').textContent = `更新于 ${new Date().toLocaleTimeString('zh-CN', { hour12: false })}`;
  $('data-state').classList.remove('stale');
}

function saveManagementKey(key) {
  try {
    localStorage.setItem(managementKeyStorage, key);
    return true;
  } catch {
    return false;
  }
}

function forgetManagementKey(expectedKey) {
  try {
    // An expired session in another tab must not remove a newly saved credential.
    if (expectedKey === undefined || localStorage.getItem(managementKeyStorage) === expectedKey) {
      localStorage.removeItem(managementKeyStorage);
    }
    return true;
  } catch {
    return false;
  }
}

function restoreManagementConnection() {
  let key;
  try {
    key = localStorage.getItem(managementKeyStorage)?.trim();
  } catch {
    notice('浏览器未允许本地存储，请输入管理密钥连接。当前连接仍可正常使用。');
    return;
  }
  if (key) void connectManagement(key, { restoring: true });
}

async function connectManagement(key, { restoring = false } = {}) {
  key = key.trim();
  if (!key) return;
  disconnect('', false, { forgetKey: false });
  client.open(key);
  const epoch = client.epoch;
  $('data-state').textContent = '正在连接管理接口…';
  $('connection-state').textContent = '连接中';
  try {
    await refresh();
    // A disconnect, navigation or newer connection must also invalidate a completed refresh.
    if (epoch !== client.epoch) return;
    // Restoring an existing key must not overwrite another tab's save or disconnect.
    const remembered = restoring || saveManagementKey(key);
    state.connected = true;
    $('connection-state').textContent = '已连接';
    document.body.classList.add('connected');
    $('connection-dialog').close();
    renderAdmission();
    renderRules();
    if (state.page === 'login') loginWorkbench.open({ focus: false });
    schedulePoll();
    if (!remembered) notice('已连接，但浏览器未能保存管理密钥。刷新后需要重新输入，请允许本站使用本地存储。');
  } catch (error) {
    if (!isDiscarded(error) && epoch === client.epoch) {
      disconnect(`连接未完成：${error.message}。已保存的密钥会保留，刷新页面可重试。`, true, { forgetKey: false });
    }
  }
}

function disconnect(
  message = '已清除管理密钥与页面数据。后台任务继续执行，重新连接后可查询或取消。',
  failed = false,
  { forgetKey = true, expectedKey } = {},
) {
  const forgotten = !forgetKey || forgetManagementKey(expectedKey);
  client.clear();
  state.connected = false;
  loginWorkbench.reset();
  state.targetsFresh = false;
  clearTimeout(state.poll);
  Object.keys(state.reads).forEach((key) => state.reads[key]++);
  Object.assign(state, {
    targets: [],
    questions: [],
    rules: [],
    actions: [],
    runs: [],
    run: null,
    release: null,
    settings: {},
    historyOffset: 0,
    historyTotal: 0,
    resourceStatus: {},
  });
  document.querySelectorAll('dialog[open]').forEach((dialog) => dialog.close());
  $('management-key').value = '';
  for (const id of [
    'models',
    'prompt',
    'rule-models',
    'rule-prompt',
    'rule-review-standard',
    'release-preview',
  ])
    $(id).value !== undefined ? ($(id).value = '') : $(id).replaceChildren();
  $('safe-preview').srcdoc = '';
  $('rule-form').reset();
  $('settings-form').reset();
  $('settings-fields').replaceChildren();
  for (const id of ['action-title', 'action-description', 'action-error', 'local-feedback']) $(id).textContent = '';
  document.body.classList.remove('connected');
  $('connection-state').textContent = '未连接';
  state.releaseBlocked = true;
  state.releaseAction = null;
  state.compareRun = null;
  state.editor = null;
  actionRequest = null;
  $('action-name').value = '';
  $('run-feedback').classList.add('hidden');
  for (const id of ['compare-model-a', 'compare-model-b', 'sample-filter']) $(id).replaceChildren();
  $('run-progress').style.width = '0%';
  $('run-progress').parentElement.setAttribute('aria-valuenow', '0');
  $('evidence-content').replaceChildren();
  $('release-feedback').replaceChildren();
  $('release-feedback').classList.add('hidden');
  for (const id of ['rule-dialog', 'settings-dialog']) $(id).classList.add('hidden');
  $('operations-main').classList.remove('hidden');
  setManualView('setup');
  $('data-state').textContent = failed ? '管理连接失效，请重新连接' : '管理接口未连接';
  $('data-state').classList.add('stale');
  $('run-content').classList.add('hidden');
  $('task-actions').classList.add('hidden');
  $('run-empty').classList.remove('hidden');
  $('task-indicator').textContent = '';
  for (const id of ['run-rerun', 'run-export', 'run-cancel']) $(id).classList.add('hidden');
  for (const id of ['run-frozen-data', 'sample-list', 'run-meta', 'operations-audit', 'operations-history'])
    $(id).replaceChildren();
  $('run-subtitle').textContent = '回答会显示在这里，不自动评分。';
  renderSelectors();
  renderHistory();
  renderRules();
  renderTargets();
  renderMetrics({});
  renderActivityState();
  renderAdmission();
  if (!forgotten) notice('已断开当前连接，但浏览器未允许清除已保存的管理密钥。请清除此站点的本地存储。', true);
  else if (message) notice(message, failed);
  else clearNotice();
}

function setPage(page) {
  if (page !== state.page) clearNotice();
  state.page = ['manual', 'operations', 'login'].includes(page) ? page : 'manual';
  if (state.page === 'login') loginWorkbench.open({ focus: false });
  else loginWorkbench.close({ focus: false });
  $('page-manual').classList.toggle('hidden', state.page !== 'manual');
  $('page-operations').classList.toggle('hidden', state.page !== 'operations');
  $('page-login').classList.toggle('hidden', state.page !== 'login');
  $('page-crumb').textContent = state.page === 'manual' ? '智商测试' : state.page === 'operations' ? '质量运维' : 'OpenAI 登录';
  document.querySelectorAll('[data-page]').forEach((button) => {
    button.classList.toggle('active', button.dataset.page === state.page);
    button.setAttribute('aria-current', button.dataset.page === state.page ? 'page' : 'false');
  });
  history.replaceState(null, '', `#${state.page}`);
}

function setManualView(view, focus = false) {
  const showConfiguration = view === 'setup';
  const nextView = view === 'history' ? 'history' : 'results';
  if (nextView !== state.manualView) clearNotice();
  state.manualView = nextView;
  if (showConfiguration) setReadingExpanded(false);
  for (const name of ['results', 'history']) {
    $(`manual-${name}`).classList.toggle('hidden', name !== state.manualView);
  }
  document.querySelector('.answer-workspace')?.classList.remove('hidden');
  document.querySelectorAll('.view-nav [data-manual-view]').forEach((button) => {
    const selected = button.dataset.manualView === state.manualView;
    button.classList.toggle('active', selected);
    button.setAttribute('aria-current', selected ? 'page' : 'false');
    button.setAttribute('aria-controls', `manual-${button.dataset.manualView}`);
  });
  $('page-manual').dataset.view = state.manualView;
  $('manual-title').textContent = '智商测试';
  if (focus) {
    const section = $(showConfiguration ? 'manual-setup' : `manual-${state.manualView}`);
    section.tabIndex = -1;
    section.focus({ preventScroll: true });
    section.scrollIntoView({ block: 'nearest', behavior: 'instant' });
  }
}

function setReadingExpanded(expanded) {
  state.readingExpanded = expanded;
  $('manual-setup').classList.toggle('hidden', expanded);
  $('page-manual').dataset.readingExpanded = String(expanded);
  const toggle = $('reading-toggle');
  if (!toggle) return;
  toggle.textContent = expanded ? '返回配置' : '展开阅读';
  toggle.setAttribute('aria-pressed', String(expanded));
  toggle.setAttribute('aria-controls', 'manual-setup');
}

function setOpsView(view) {
  if (view !== state.opsView) clearNotice();
  state.opsView = ['attention', 'rules', 'activity'].includes(view) ? view : 'attention';
  for (const name of ['attention', 'rules', 'activity']) $(`ops-${name}`).classList.toggle('hidden', name !== state.opsView);
  document.querySelectorAll('[data-ops-view]').forEach((button) => {
    const selected = button.dataset.opsView === state.opsView;
    button.classList.toggle('active', selected);
    button.setAttribute('aria-current', selected ? 'page' : 'false');
  });
}

function openEditor(id) {
  loginWorkbench.close({ focus: false });
  state.reads.editor++;
  state.editor = { id, snapshot: editorSnapshot(id), opener: document.activeElement };
  if (!$(id).querySelector('.editor-dirty')) {
    const indicator = document.createElement('span');
    indicator.className = 'editor-dirty';
    indicator.setAttribute('role', 'status');
    $(id).querySelector('.editor-footer, .button-row.end').prepend(indicator);
  }
  updateEditorDirty();
  $('operations-main').classList.add('hidden');
  for (const name of ['rule-dialog', 'settings-dialog']) $(name).classList.toggle('hidden', name !== id);
  const heading = $(id).querySelector('h2');
  heading.tabIndex = -1;
  heading.focus({ preventScroll: true });
  $(id).scrollIntoView({ block: 'start' });
}

function closeSurface(id, saved = false) {
  const node = $(id);
  if (node.tagName === 'DIALOG') { if (id !== 'action-dialog' || !state.busy.has('dialog-action')) node.close(); }
  else {
    if (!saved && state.editor?.saving) return;
    if (!saved && editorDirty()) { protectEditor(() => closeSurface(id, true)); return; }
    node.classList.add('hidden');
    $('operations-main').classList.remove('hidden');
    const opener = state.editor?.opener?.isConnected ? state.editor.opener : id === 'rule-dialog' ? $('rule-new') : $('settings-open');
    state.editor = null;
    opener.focus({ preventScroll: true });
  }
}

function updatePlan() {
  const samples = number('samples');
  $('run-plan').textContent = value('models') && value('prompt') && Number.isInteger(samples) && samples >= 1 && samples <= 8
    ? `预计生成 ${samples} 份独立回答`
    : '';
  $('run-plan').hidden = !$('run-plan').textContent;
}

function targetOptions(includeBlank = true) {
  return `${includeBlank ? '<option value="">选择凭证</option>' : ''}${state.targets.map((target) => `<option value="${escapeHTML(targetID(target))}">${escapeHTML(target.provider || 'unknown')} · ${escapeHTML(target.auth_index || targetID(target))}${target.quality_paused ? ' · 质量暂停' : ''}${target.manual_disabled ? ' · 人工禁用' : ''}</option>`).join('')}`;
}

function questionOptions() {
  return (
    state.questions
      .map(
        (question) =>
          `<option value="${escapeHTML(question.id)}">${escapeHTML(question.title || question.id)} · ${escapeHTML(question.kind || '')}</option>`,
      )
      .join('') +
    (state.questions.some((question) => question.id === 'custom')
      ? ''
      : '<option value="custom">自定义题目</option>')
  );
}

function renderSelectors() {
  const previousQuestion = value('question');
  const previousPrompt = $('prompt').value;
  for (const id of ['target', 'rule-target', 'rule-judge-target', 'rule-control-target']) {
    const previous = $(id).value;
    $(id).innerHTML = targetOptions();
    if (findTarget(previous)) $(id).value = previous;
  }
  const curated = ['logic-candy', 'visual-pelican-html', 'custom'];
  const questions = curated.map((id) => state.questions.find((question) => question.id === id)).filter(Boolean);
  $('question').innerHTML = questions.length
    ? questions.map((question) => `<option value="${escapeHTML(question.id)}">${escapeHTML(question.title || question.id)}</option>`).join('')
    : '<option value="">连接后加载题目</option>';
  if (questions.some((question) => question.id === previousQuestion)) $('question').value = previousQuestion;
  const ruleQuestion = value('rule-question');
  $('rule-question').innerHTML = questionOptions();
  if ([...$('rule-question').options].some((option) => option.value === ruleQuestion)) $('rule-question').value = ruleQuestion;
  renderTarget();
  renderQuestion();
  if (previousQuestion && value('question') === previousQuestion) {
    $('prompt').value = previousPrompt;
    updateQuestionNote();
  }
}

function availableModels(target) {
  return [...new Set((target?.models || []).map((entry) => typeof entry === 'string' ? entry : entry.id || entry.model).filter(Boolean))];
}

function renderReasoning(resetInvalid = false) {
  const target = findTarget(value('target'));
  const model = value('models');
  const validModel = availableModels(target).includes(model);
  const capability = validModel ? target.reasoning?.[model] : null;
  const labels = { none: '关闭思考', minimal: '最低', low: '低', medium: '中', high: '高', xhigh: '极高', max: '最高', auto: '自动' };
  const levels = capability?.Levels ?? capability?.levels ?? [];
  const supported = new Set(Array.isArray(levels) ? levels.filter((level) => Object.hasOwn(labels, level)) : []);
  if (capability?.ZeroAllowed ?? capability?.zero_allowed) supported.add('none');
  if (capability?.DynamicAllowed ?? capability?.dynamic_allowed) supported.add('auto');
  const previous = value('reasoning');
  const unavailable = !!previous && !supported.has(previous) && !resetInvalid;
  const options = '<option value="">默认</option>' + (unavailable ? `<option value="${escapeHTML(previous)}" disabled>原设置 ${escapeHTML(previous)} 已不可用</option>` : '') + Object.entries(labels).filter(([level]) => supported.has(level)).map(([level, label]) => `<option value="${level}">${label}</option>`).join('');
  if ($('reasoning').innerHTML !== options) $('reasoning').innerHTML = options;
  $('reasoning').value = supported.has(previous) || unavailable ? previous : '';
  $('reasoning').disabled = !supported.size && !unavailable;
  $('reasoning').dataset.unavailable = String(unavailable);
  const note = !validModel ? ''
    : unavailable ? '原思考强度已不在支持范围，请重新选择。'
    : !capability ? '思考能力未知，使用模型默认设置。'
    : !supported.size ? '未提供可选思考档位，使用模型默认设置。'
    : '仅显示此模型支持的思考强度。';
  $('reasoning-note').textContent = note;
  $('reasoning-note').hidden = !note || (validModel && !!supported.size && !unavailable);
  $('reasoning-note').classList.toggle('error-note', unavailable);
}

function renderTarget(selectModel = false) {
  const target = findTarget(value('target'));
  const models = availableModels(target);
  const previous = value('models');
  const unavailable = !!target && !!previous && !models.includes(previous) && !selectModel;
  const selected = models.includes(previous) || unavailable ? previous : models[0] || '';
  const placeholder = !target ? '先选择凭证' : '此凭证暂无可用模型';
  const options = (unavailable ? `<option value="${escapeHTML(previous)}" disabled>${escapeHTML(previous)}（已不可用）</option>` : '') + (models.length ? models.map((model) => `<option value="${escapeHTML(model)}">${escapeHTML(model)}</option>`).join('') : `<option value="">${placeholder}</option>`);
  if ($('models').innerHTML !== options) $('models').innerHTML = options;
  $('models').value = selected;
  $('models').disabled = !models.length;
  $('models').title = selected;
  $('model-note').textContent = !target ? ''
    : unavailable ? '原模型已从目录移除，请重新选择模型。'
    : models.length ? `可选 ${models.length} 个模型` : '未发现已注册模型，请在 CPA 检查配置后刷新数据。';
  $('model-note').classList.toggle('error-note', !!target && (!models.length || unavailable));
  if (!target) {
    $('target').removeAttribute('title');
    $('target-summary').textContent = state.targets.length ? '选择要测试的凭证。' : resourceMessage('targets', '测试凭证') || '尚无可用凭证，请先在 CPA 中添加凭证。';
  } else {
    const reason = credentialBlock({ auth_id: targetID(target) }, state.targets);
    $('target').title = `auth_id: ${targetID(target)} · auth_index: ${target.auth_index || '未知'}`;
    patchView($('target-summary'), reason ? `<span class="error-note">${escapeHTML(reason)}</span>` : '<span>凭证可用</span>');
  }
  renderReasoning(selectModel);
  renderAdmission();
}

function updateQuestionNote() {
  const question = state.questions.find((entry) => entry.id === value('question'));
  const selected = questionTestSelection(question, $('prompt').value);
  const changed = !!question && question.id !== 'custom' && selected.question_id === 'custom';
  $('question-version').textContent = changed ? '已修改' : question?.id === 'custom' ? '' : question ? `版本 ${question.version || '未知'}` : '';
  const note = $('prompt-note');
  note.hidden = !changed;
  note.textContent = changed ? '按自定义题目测试，不沿用参考答案。' : '';
}

function renderQuestion() {
  const question = state.questions.find((entry) => entry.id === value('question'));
  $('prompt').readOnly = false;
  $('prompt').value = question?.prompt || '';
  updateQuestionNote();
  renderAdmission();
}

function manualRunConfig() {
  const target = findTarget(value('target'));
  if (!target) fieldError('target', '请选择要测试的凭证。');
  const models = parseModels(value('models'));
  if (models.length !== 1 || !availableModels(target).includes(models[0])) fieldError('models', '请选择此凭证目录中的可用模型。');
  if ($('reasoning').dataset.unavailable === 'true') fieldError('reasoning', '请重新选择可用的思考强度。');
  const samples = number('samples');
  if (!Number.isInteger(samples) || samples < 1 || samples > 8) fieldError('samples', '回答数量须为 1–8 的整数。');
  if (!$('prompt').value.trim()) fieldError('prompt', '请填写测试题目。');
  const question = state.questions.find((entry) => entry.id === value('question'));
  const config = {
    auth_id: targetID(target), provider: target.provider, models, samples,
    ...questionTestSelection(question, $('prompt').value),
    parameters: { stream: true },
    judge: { mode: 'manual' },
  };
  if (value('reasoning')) config.parameters.reasoning_effort = value('reasoning');
  return config;
}

function runConfig(prefix = '') {
  if (!prefix) return manualRunConfig();
  const get = (name) => value(`${prefix}${name}`);
  const target = findTarget(get('target'));
  if (!target) fieldError(`${prefix}target`, '请选择目标凭证。');
  const models = parseModels(get('models'));
  if (!models.length || models.length > 8) fieldError(`${prefix}models`, '请指定 1–8 个模型。');
  const judge = { mode: get('judge-mode') };
  if (judge.mode === 'llm') {
    const selected = findTarget(get('judge-target'));
    if (!selected || targetID(selected) === targetID(target))
      fieldError(`${prefix}judge-target`, '判题需要独立于被测凭证的另一凭证。');
    if (!get('judge-model')) fieldError(`${prefix}judge-model`, '请输入判题模型。');
    Object.assign(judge, {
      auth_id: targetID(selected),
      provider: selected.provider,
      model: get('judge-model'),
    });
  }
  const config = {
    auth_id: targetID(target),
    provider: target.provider,
    models,
    question_id: get('question'),
    samples: number(`${prefix}samples`),
    parameters: {
      temperature: number(`${prefix}temperature`),
      max_tokens: number(`${prefix}max-tokens`),
      stream: true,
    },
    judge,
  };
  if (config.question_id === 'random-linear' && get('control-target')) {
    const control = findTarget(get('control-target'));
    if (!control || targetID(control) === targetID(target))
      fieldError(`${prefix}control-target`, '随机题通过对照必须选择另一个凭证。');
    config.control_auth_id = targetID(control);
    config.control_provider = control.provider;
  }
  if (get('reasoning')) config.parameters.reasoning_effort = get('reasoning');
  if (config.question_id === 'custom') {
    config.review_standard = get('review-standard');
    config.prompt = get('prompt');
    if (!config.prompt) fieldError(`${prefix}prompt`, '自定义题面不能为空。');
  }
  return config;
}

function blockedConfig(config) {
  if (!state.connected) return '请先连接管理接口。';
  if (!state.targetsFresh) return '凭证状态尚未确认，请刷新后再执行。';
  return credentialBlock(config, state.targets);
}

function renderAdmission() {
  const config = {
    auth_id: value('target'),
    judge: { mode: 'manual' },
  };
  const blocked = blockedConfig(config) || (config.auth_id && !availableModels(findTarget(config.auth_id)).includes(value('models')) ? '请先选择可用的测试模型。' : '') || ($('reasoning').dataset.unavailable === 'true' ? '请重新选择可用的思考强度。' : '');
  $('run-submit').disabled = !config.auth_id || !!blocked || !value('prompt') || state.busy.has('submit-run');
  $('run-submit').title = state.connected ? blocked : '';
  $('run-admission').textContent = state.connected ? blocked || '不自动评分，也不会暂停凭证。' : '';
  $('run-admission').hidden = !state.connected;
  $('run-guidance').hidden = !state.connected;
  $('run-admission').classList.toggle('error-note', !!blocked && state.connected);
  $('release-confirm').disabled = state.releaseBlocked || state.busy.has('release') || !state.connected;
  updatePlan();
  for (const id of [
    'rule-new',
    'settings-open',
    'run-export',
    'run-cancel',
    'refresh',
  ])
    $(id).disabled = !state.connected || $(id).getAttribute('aria-busy') === 'true';
  if (state.run) {
    const reason = blockedConfig(state.run.config || {});
    $('run-rerun').disabled = !!reason || state.busy.has('submit-run');
    $('run-rerun').title = reason;
  }
}

function newKey() {
  return globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

async function submitRun(config) {
  const blocked = blockedConfig(config);
  if (blocked) throw new Error(blocked);
  const result = await api('/runs', 'POST', { ...config, idempotency_key: newKey() });
  notice('任务已入队。离开页面不会停止后台任务。');
  await loadRun(result.run_id || result.id);
  await loadHistory();
}

async function loadRun(id, show = true) {
  if (!id) throw new Error('接口未返回任务 ID。');
  const sequence = ++state.reads.run;
  clearTimeout(state.poll);
  const run = await api(`/runs/detail?id=${encodeURIComponent(id)}`);
  if (sequence !== state.reads.run) return;
  state.run = run.run || run;
  state.pollFailures = 0;
  renderRun();
  if (show) { clearNotice(); setManualView('results', true); }
  schedulePoll();
}

function schedulePoll() {
  clearTimeout(state.poll);
  if (!state.connected || !active(state.run)) return;
  const epoch = client.epoch;
  const sequence = state.reads.run;
  const id = state.run.id || state.run.run_id;
  const delay = Math.min(10000, 1200 * 2 ** state.pollFailures);
  state.poll = setTimeout(async () => {
    if (epoch !== client.epoch || sequence !== state.reads.run) return;
    try {
      const reply = await api(`/runs/detail?id=${encodeURIComponent(id)}`);
      if (sequence !== state.reads.run) return;
      state.run = reply.run || reply;
      $('run-feedback').classList.add('hidden');
      state.pollFailures = 0;
      renderRun();
      if (active(state.run)) schedulePoll();
      else await Promise.all([loadHistory(), loadOperations()]);
    } catch (error) {
      if (isDiscarded(error) || epoch !== client.epoch || sequence !== state.reads.run) return;
      state.pollFailures = Math.min(state.pollFailures + 1, 4);
      $('run-feedback').textContent = `进度查询失败：${error.message}。会自动重试查询，不会重新提交模型任务。`;
      $('run-feedback').classList.remove('hidden');
      schedulePoll();
    }
  }, delay);
}

// Reconcile trusted view templates without replacing stable reading and editing nodes.
function patchView(target, html) {
  const template = document.createElement('template');
  template.innerHTML = html;
  const key = (node) => node.nodeType === 1 ? node.dataset.key || node.dataset.part ||
    (node.matches('input,select') && node.closest('.sample-review') ? node.localName : node.id) || '' : '';
  function patchChildren(parent, incoming) {
    const existing = [...parent.childNodes];
    const used = new Set();
    let cursor = parent.firstChild;
    const nextNodes = [...incoming.childNodes];
    if (parent.nodeType === 1 && parent.matches('.comparison-grid')) {
      nextNodes.sort((a, b) => {
        const position = (node) => { const index = existing.findIndex((old) => key(old) === key(node)); return index < 0 ? existing.length : index; };
        return position(a) - position(b);
      });
    }
    for (const next of nextNodes) {
      const nextKey = key(next);
      const current = existing.find((node) => !used.has(node) && node.nodeType === next.nodeType &&
        (nextKey ? key(node) === nextKey : !key(node) && node.nodeName === next.nodeName));
      if (!current) { parent.insertBefore(next.cloneNode(true), cursor); continue; }
      used.add(current);
      if (current !== cursor) parent.insertBefore(current, cursor);
      cursor = current.nextSibling;
      if (current.nodeType === 3) {
        if (current.nodeValue !== next.nodeValue) current.nodeValue = next.nodeValue;
        continue;
      }
      if (current.nodeType !== 1) continue;
      const field = current.matches('input,select,textarea');
      const fieldValue = field ? current.value : null;
      const preservedClasses = ['expanded', ...(current.matches('.model-result,.sample') ? ['hidden'] : [])].filter((name) => current.classList.contains(name));
      for (const attribute of [...current.attributes]) {
        if (['open', 'style', 'aria-busy'].includes(attribute.name)) continue;
        if (!next.hasAttribute(attribute.name)) current.removeAttribute(attribute.name);
      }
      for (const attribute of [...next.attributes]) {
        if (attribute.name === 'open') continue;
        if (current.getAttribute(attribute.name) !== attribute.value) current.setAttribute(attribute.name, attribute.value);
      }
      current.classList.add(...preservedClasses);
      const expandedButton = current.matches('[data-expand-sample]') && current.closest('.sample')?.querySelector('.answer-text.expanded');
      if (expandedButton) current.setAttribute('aria-expanded', 'true');
      else patchChildren(current, next);
      if (field) current.value = fieldValue;
    }
    existing.filter((node) => !used.has(node)).forEach((node) => node.remove());
  }
  patchChildren(target, template.content);
}

function renderRun() {
  const run = state.run;
  if (!run) return;
  $('run-empty').classList.add('hidden');
  $('run-content').classList.remove('hidden');
  $('task-actions').classList.remove('hidden');
  const repeatable = run.source === 'manual_test' && run.config?.models?.length === 1 && run.config?.judge?.mode === 'manual' && !run.config?.control_auth_id;
  $('run-rerun').classList.toggle('hidden', active(run) || !repeatable);
  $('run-export').classList.remove('hidden');
  $('run-cancel').classList.toggle('hidden', !active(run));
  const rendered = renderRunEvidence(run);
  $('run-subtitle').textContent = `${run.source === 'manual_test' ? '题目测试 · 不自动评分' : run.source === 'scheduled_probe' ? '规则检测 · 依据规则许可处置' : sourceLabel(run.source)} · ${date(run.created_at)}`;
  patchView($('run-meta'), rendered.metaHTML);
  patchView($('run-frozen-data'), `<div class="run-identifier">任务 ${escapeHTML(run.id || run.run_id)}</div>${rendered.conditionsHTML}`);
  const progress = Math.min(100, Math.round(rendered.completed / Math.max(1, rendered.planned) * 100));
  $('run-progress').style.width = `${progress}%`;
  $('run-progress').parentElement.setAttribute('aria-valuenow', String(progress));
  $('task-indicator').textContent = active(run) ? '执行中' : '已结束';
  const id = run.id || run.run_id;
  const sameRun = state.compareRun === id;
  if (sameRun) patchView($('sample-list'), rendered.samplesHTML);
  else $('sample-list').innerHTML = rendered.samplesHTML;
  if (!sameRun) {
    const models = [...new Set([...(run.config?.models || []), ...(run.samples || []).map((sample) => sample.model || sample.request_model || '模型未知')])];
    const options = models.map((model, index) => `<option value="${escapeHTML(model)}">${index + 1} · ${escapeHTML(model.length > 48 ? `${model.slice(0,18)}…${model.slice(-24)}` : model)}</option>`).join('');
    $('compare-model-a').innerHTML = options;
    $('compare-model-b').innerHTML = '<option value="">不对比</option>' + options;
    $('compare-model-b').value = models[1] || '';
    $('sample-filter').innerHTML = '<option value="all">全部样本</option>' + Array.from({length: run.config?.samples || 1}, (_, index) => `<option value="${index}">样本 ${index + 1}</option>`).join('');
    document.querySelector('.model-selectors').classList.toggle('hidden', models.length < 2);
    document.querySelector('.comparison-toolbar').classList.toggle('hidden', models.length < 2 && ((run.source === 'manual_test' && run.config?.judge?.mode === 'manual') || (run.config?.samples || 1) < 2));
    state.compareRun = id;
    $('run-frozen').open = false;
    $('run-feedback').classList.add('hidden');
  }
  applyComparison();
  renderAdmission();
}

function applyComparison() {
  const selected = [value('compare-model-a'), value('compare-model-b')].filter(Boolean);
  const filter = value('sample-filter');
  const grid = $('sample-list').querySelector('.comparison-grid');
  if (!grid) return;
  const order = [...selected, ...[...grid.children].map((group) => group.dataset.model).filter((model) => !selected.includes(model))];
  for (const [index, model] of order.entries()) {
    const group = [...grid.children].find((node) => node.dataset.model === model);
    if (group && grid.children[index] !== group) grid.insertBefore(group, grid.children[index]);
  }
  let visible = 0;
  for (const group of grid.children) {
    const shown = selected.includes(group.dataset.model);
    group.classList.toggle('hidden', !shown);
    if (shown) visible++;
    for (const sample of group.querySelectorAll('.sample')) sample.classList.toggle('hidden', filter !== 'all' && sample.dataset.sample !== filter);
  }
  grid.classList.toggle('single-model', visible < 2);
  const visibleGroup = [...grid.children].find((node) => !node.classList.contains('hidden'));
  grid.style.setProperty('--comparison-rows', String(1 + (visibleGroup?.querySelectorAll('.sample:not(.hidden)').length || 1)));
}

function resourceMessage(resource, label) {
  const status = state.resourceStatus[resource];
  if (status === 'ready') return '';
  if (status === 'loading') return `正在读取${label}…`;
  if (status === 'error') return `${label}读取失败，请刷新重试。`;
  return state.connected ? `尚未加载${label}，请刷新读取。` : `连接管理接口后读取${label}。`;
}

function renderActivityState() {
  for (const [id, resource, label] of [['operations-history', 'activity', '规则运行记录'], ['operations-audit', 'audit', '审计记录']]) {
    const message = resourceMessage(resource, label);
    if (message && !$(id).querySelector('.history-row')) patchView($(id), `<p class="empty-line">${escapeHTML(message)}</p>`);
  }
}

function renderHistory() {
  $('history-total').textContent = `${state.historyTotal} 个任务`;
  $('history-count').textContent = state.historyTotal || '';
  patchView($('history-list'), state.runs.length
    ? state.runs
        .map(
          (run) =>
            `<div class="history-row" role="button" tabindex="0" data-key="run:${escapeHTML(run.id || run.run_id)}" data-run="${escapeHTML(run.id || run.run_id)}"><div><div class="history-title">${escapeHTML(run.config?.models?.join(' / ') || run.rule_id || '智商测试')}</div><div class="history-time">${escapeHTML(date(run.created_at))} · ${escapeHTML(sourceLabel(run.source))} · ${escapeHTML(run.config?.auth_id || '')}</div></div>${statusTag(run.status)}</div>`,
        )
        .join('')
    : `<p class="empty-line">${escapeHTML(resourceMessage('history', '测试记录') || '暂无测试记录。首次执行后在这里查看。')}</p>`);
  $('history-page').textContent = `第 ${Math.floor(state.historyOffset / 10) + 1} 页`;
  $('history-prev').disabled = state.historyOffset === 0;
  $('history-next').disabled = state.historyOffset + 10 >= state.historyTotal;
  $('history-pagination').hidden = state.historyTotal <= 10 && state.historyOffset === 0;
}

async function loadHistory() {
  const sequence = ++state.reads.history;
  if (state.resourceStatus.history !== 'ready') state.resourceStatus.history = 'loading';
  if (!state.runs.length) renderHistory();
  let result;
  try {
    result = await api(`/runs?source=manual_test&offset=${state.historyOffset}&limit=10`);
  } catch (error) {
    if (sequence === state.reads.history && !isDiscarded(error)) {
      state.resourceStatus.history = 'error';
      renderHistory();
    }
    throw error;
  }
  if (sequence !== state.reads.history) return;
  state.resourceStatus.history = 'ready';
  state.runs = items(result);
  state.historyTotal = result.total ?? state.runs.length;
  renderHistory();
}

function renderRules() {
  const message = !state.rules.length && resourceMessage('rules', '检测规则');
  patchView($('rules-list'), message ? `<tr><td colspan="5" class="empty-line">${escapeHTML(message)}</td></tr>` : renderRuleRows(state.rules, state.targets, state));
  $('rule-count').textContent = state.rules.length || '';
}

function renderTargets() {
  const targetMessage = !state.targets.length && resourceMessage('targets', '凭证限制');
  const actionMessage = !state.actions.length && resourceMessage('actions', '宿主保存的暂停归属');
  patchView($('targets-list'), targetMessage ? `<tr><td colspan="5" class="empty-line">${escapeHTML(targetMessage)}</td></tr>` : renderTargetRows(state.targets));
  patchView($('actions-list'), actionMessage ? `<p class="empty-line">${escapeHTML(actionMessage)}</p>` : renderActionItems(state.actions, state.targets));
  $('attention-count').textContent = state.actions.length || '';
}

function renderMetrics(metrics) {
    const percentage = (number) =>
    number == null ? '—' : `${(number <= 1 ? number * 100 : number).toFixed(1)}%`;
  const stats = [
    [
      '质量通过率',
      percentage(metrics.quality_pass_rate ?? metrics.pass_rate),
      `正确 ${metrics.correct ?? '—'} / 可判定 ${metrics.judged ?? (metrics.correct != null && metrics.incorrect != null ? metrics.correct + metrics.incorrect : '—')}`,
    ],
    [
      '评测覆盖率',
      percentage(metrics.coverage ?? metrics.coverage_rate),
      `错误 ${metrics.incorrect ?? '—'} · 未知 ${metrics.unknown ?? '—'}`,
    ],
    ['质量暂停归属', state.resourceStatus.actions === 'ready' ? state.actions.length : '—', '对凭证所有模型的新业务生效'],
    [
      '已知用量',
      (metrics.known_tokens ?? metrics.total_tokens) == null
        ? '—'
        : Number(metrics.known_tokens ?? metrics.total_tokens).toLocaleString(),
      `usage 未知 ${metrics.unknown_usage ?? '—'} · 丢事件 ${metrics.dropped_usage_events ?? metrics.dropped_events ?? '—'}`,
    ],
  ];
  $('metrics-detail').textContent = resourceMessage('metrics', '质量统计') || `执行错误：${
    Object.entries(metrics.error_types || {})
      .map(([kind, count]) => `${statusLabels[kind] || kind} ${count}`)
      .join(' · ') || '暂无已记录错误'
  }。生产 usage 表示执行尝试，不等同于入站请求。缺失来源不归为生产。`;
  $('metrics').innerHTML = stats
    .map(
      ([name, total, subtitle]) =>
        `<article class="metric"><span>${escapeHTML(name)}</span><strong>${escapeHTML(total)}</strong><small>${escapeHTML(subtitle)}</small></article>`,
    )
    .join('');
}

async function loadOperations() {
  const sequence = ++state.reads.operations;
  const epoch = client.epoch;
  const resources = ['rules', 'actions', 'metrics', 'activity', 'audit', 'targets'];
  for (const resource of resources) if (state.resourceStatus[resource] !== 'ready') state.resourceStatus[resource] = 'loading';
  if (!state.rules.length) renderRules();
  if (!state.actions.length || !state.targets.length) renderTargets();
  renderActivityState();
  if (state.resourceStatus.metrics === 'loading') renderMetrics({});
  const results = await Promise.allSettled([
    api('/rules?limit=100'),
    api('/actions?limit=100'),
    api('/metrics'),
    api('/runs?limit=20&source=scheduled_probe'),
    api('/history?limit=20'),
    api('/targets?limit=100'),
  ]);
  if (epoch !== client.epoch || sequence !== state.reads.operations) return;
  results.forEach((result, index) => { state.resourceStatus[resources[index]] = result.status === 'fulfilled' ? 'ready' : 'error'; });
  state.targetsFresh = results[5].status === 'fulfilled';
  if (results[5].status === 'fulfilled') {
    state.targets = items(results[5].value);
    renderTarget();
  }
  if (results[0].status === 'fulfilled') {
    state.rules = items(results[0].value);
    renderRules();
  }
  if (results[1].status === 'fulfilled') {
    state.actions = items(results[1].value);
    renderTargets();
  }
  if (results[2].status === 'fulfilled') renderMetrics(results[2].value);
  if (results[3].status === 'fulfilled') {
    const runs = items(results[3].value).filter((run) => run.source !== 'manual_test');
    patchView($('operations-history'), runs.length
      ? runs
          .map(
            (run) =>
              `<div class="history-row" role="button" tabindex="0" data-key="run:${escapeHTML(run.id || run.run_id)}" data-run="${escapeHTML(run.id || run.run_id)}"><div><div class="history-title">${escapeHTML(run.rule_id || '规则检测')} · ${escapeHTML(statusLabels[run.action_status] || run.action_status || '仅观察')}</div><div class="history-time">${escapeHTML(date(run.created_at))} · ${escapeHTML(run.error || run.reason || '')}</div></div>${statusTag(run.status)}</div>`,
          )
          .join('')
      : results[4].status === 'fulfilled' && items(results[4].value).length === 0
        ? '<p class="empty-line">暂无运营记录。自动处置不会使用手动智商测试结果。</p>'
        : '<p class="empty-line">暂无规则运行记录。</p>');
  }
  if (results[4].status === 'fulfilled') {
    const audits = items(results[4].value);
    patchView($('operations-audit'), audits
      .map(
        (audit) =>
          `<div class="history-row audit-row" data-key="audit:${escapeHTML(audit.id || JSON.stringify([audit.time, audit.kind, audit.run_id, audit.action_id]))}"><div><div class="history-title">${escapeHTML(audit.kind || 'audit')} · ${escapeHTML(audit.rule_id || audit.run_id || audit.action_id || '')}</div><div class="history-time">${escapeHTML(date(audit.time))} · ${escapeHTML(audit.detail || '')}</div></div>${tag('审计')}</div>`,
      )
      .join('') || '<p class="empty-line">暂无审计记录。</p>');
  }
  renderActivityState();
  if (!state.rules.length) renderRules();
  if (!state.targets.length || !state.actions.length) renderTargets();
  if (state.resourceStatus.metrics === 'error') $('metrics-detail').textContent = '质量统计读取失败，已显示数据可能过期；请刷新重试。';
  const failed = results.find((result) => result.status === 'rejected');
  if (failed) {
    $('data-state').textContent = '部分数据更新失败，请重试';
    $('data-state').classList.add('stale');
    renderAdmission();
    throw failed.reason;
  }
  markFresh();
}

async function refresh() {
  const sequence = ++state.reads.refresh;
  try {
    const [targets, questions] = await Promise.all([api('/targets?limit=100'), api('/questions')]);
    if (sequence !== state.reads.refresh) return;
    state.targetsFresh = true;
    state.resourceStatus.targets = 'ready';
    state.targets = items(targets);
    state.questions = items(questions);
    renderSelectors();
    renderTargets();
    await Promise.all([loadHistory(), loadOperations()]);
    if (state.run) await loadRun(state.run.id || state.run.run_id, false);
  } catch (error) {
    if (!isDiscarded(error) && sequence === state.reads.refresh) {
      state.targetsFresh = false;
      $('data-state').textContent = '数据更新失败，请重试';
      $('data-state').classList.add('stale');
      renderRules();
      renderAdmission();
    }
    throw error;
  }
}

function editRule(rule = {}) {
  const config = rule.config || rule;
  $('rule-dialog-title').textContent = rule.id ? '编辑检测规则' : '新建检测规则';
  const values = {
    id: rule.id || '',
    revision: rule.revision || '',
    name: rule.name || '',
    target: config.auth_id || value('target'),
    models: config.models?.join('\n') || value('models'),
    question: config.question_id || value('question'),
    samples: config.samples || 1,
    'max-tokens': config.parameters?.max_tokens || 2048,
    temperature: config.parameters?.temperature ?? 0,
    reasoning: config.parameters?.reasoning_effort || '',
    prompt: config.prompt || '',
    'review-standard': config.review_standard || '',
    'judge-mode': config.judge?.mode || 'deterministic',
    'judge-target': config.judge?.auth_id || '',
    'control-target': config.control_auth_id || '',
    'judge-model': config.judge?.model || '',
    action: 'observe_only',
    interval: rule.interval_seconds || 3600,
    cron: rule.cron || '',
    timezone: rule.timezone || 'Asia/Shanghai',
    fail: rule.fail_threshold || 3,
    retention: rule.retention_days || 30,
  };
  for (const [name, current] of Object.entries(values)) $(`rule-${name}`).value = current;
  for (const [name, current] of Object.entries({
    enabled: rule.enabled ?? true,
    scheduling: !!rule.scheduling_enabled,
    'auto-pause': false,
  }))
    $(`rule-${name}`).checked = current;
  $('rule-control-fields').classList.toggle('hidden', value('rule-question') !== 'random-linear');
  const held = ruleHeld(rule);
  $('rule-enabled').disabled = held;
  if (held) $('rule-enabled').checked = false;
  $('rule-hold-note').classList.toggle('hidden', !held);
  setPage('operations');
  setOpsView('rules');
  openEditor('rule-dialog');
}

async function saveRule() {
  const config = runConfig('rule-');
  const body = {
    ...config,
    name: value('rule-name'),
    action: value('rule-action'),
    enabled: checked('rule-enabled'),
    scheduling_enabled: checked('rule-scheduling'),
    automatic_pause: false,
    interval_seconds: number('rule-interval'),
    cron: value('rule-cron'),
    timezone: value('rule-timezone'),
    fail_threshold: number('rule-fail'),
    retention_days: number('rule-retention'),
  };
  if (value('rule-id')) Object.assign(body, { id: value('rule-id'), revision: number('rule-revision') });
  await api('/rules', body.id ? 'PUT' : 'POST', body);
  closeSurface('rule-dialog', true);
  if (state.page === 'operations') notice(`规则“${body.name}”已保存，不会启动模型调用。`);
  await loadOperations();
}

function releaseFeedback(message, error = false) {
  $('release-feedback').textContent = message;
  $('release-feedback').classList.remove('hidden');
  $('release-feedback').classList.toggle('error', error);
}

async function previewAction(index) {
  const action = state.actions[index];
  if (!action) return;
  const sequence = ++state.reads.preview;
  state.releaseBlocked = true;
  state.releaseAction = action;
  state.release = null;
  $('release-preview').textContent = '正在读取暂停归属与凭证限制…';
  $('release-feedback').classList.add('hidden');
  $('release-refresh').classList.add('hidden');
  $('release-confirm').textContent = '确认解除，保持检测停止';
  renderAdmission();
  $('release-dialog').showModal();
  const release = { rule_id: action.rule_id, action_id: action.action_id || action.id, action_revision: action.action_revision };
  try {
    const preview = await api('/actions/preview', 'POST', release);
    if (sequence !== state.reads.preview) return;
    state.release = release;
    $('release-preview').innerHTML = renderReleasePreview(preview, action);
    state.releaseBlocked = preview.can_release === false || preview.blocked === true || preview.conflict === true;
    renderAdmission();
    return true;
  } catch (error) {
    if (isDiscarded(error) || sequence !== state.reads.preview) return;
    $('release-preview').textContent = '未取得有效解除预览。';
    releaseFeedback(`预览未完成：${error.message}。请刷新并核对当前归属。`, true);
    $('release-refresh').classList.remove('hidden');
  }
  renderAdmission();
  return false;
}

async function reconcileRelease() {
  const sequence = state.reads.preview;
  const action = { ...state.releaseAction };
  const current = () => sequence === state.reads.preview && $('release-dialog').open;
  state.releaseBlocked = true;
  renderAdmission();
  try {
    await loadOperations();
  } catch (error) {
    if (!isDiscarded(error) && current()) releaseFeedback(`状态核对失败：${error.message}。解除结果仍待核对，请再次刷新。`, true);
    renderAdmission();
    return;
  }
  if (!current()) return;
  const id = action.action_id || action.id;
  const index = state.actions.findIndex((action) => (action.action_id || action.id) === id);
  if (index >= 0) {
    const updated = await previewAction(index);
    if (!updated) return;
    releaseFeedback('已重新读取当前动作版本。请核对更新后的预览，再决定是否解除。');
    notice('暂停预览已更新，请核对当前版本。');
  } else {
    const target = findTarget(action.auth_id);
    const reasons = target?.reasons || target?.unavailable_reasons || [];
    const remaining = [target?.manual_disabled ? '人工禁用仍生效' : '', target?.quality_paused ? '仍有质量暂停' : '', ...reasons].filter(Boolean);
    $('release-preview').innerHTML = renderReleasePreview({ action, targets: state.targets }, action);
    releaseFeedback(`宿主当前列表中已无此暂停归属。${remaining.length ? `剩余限制：${remaining.join('；')}。` : target?.effective_available === true ? '宿主当前报告凭证可用。' : '凭证是否可用仍需核对。'}相关检测规则仍停止，恢复检测须另行显式操作。`);
    notice('已核对宿主状态：原暂停归属已不在活动列表；相关检测规则仍停止，请查看剩余限制。');
    $('release-confirm').textContent = '此归属已不在活动列表';
    state.release = null;
    renderAdmission();
  }
}

const budgetFields = [
  ['max_models', '最大模型数', 1, 16, 8],
  ['max_samples', '总样本上限', 1, 128, 32],
  ['max_output_tokens', '输出 token 上限', 16, 32768, 4096],
  ['max_response_bytes', '响应字节上限', 1024, 2097152, 262144],
  ['daily_calls', '每日调用预算', 1, 100000, 200],
  ['daily_tokens', '每日 token 预算', 1, 100000000, 1000000],
  ['retention_days', '历史保留（天）', 1, 365, 30],
  ['max_storage_bytes', '存储上限（字节）', 1048576, 107374182400, 134217728],
];

async function openSettings() {
  const sequence = ++state.reads.editor;
  const editor = state.editor;
  const settings = await api('/settings');
  if (sequence !== state.reads.editor || editor !== state.editor || state.page !== 'operations') return;
  state.settings = settings;
  $('settings-scheduling').checked = !!state.settings.scheduling_enabled;
  $('settings-auto-pause').checked = false;
  $('settings-auto-pause').disabled = true;
  $('settings-fields').innerHTML =
    budgetFields
      .map(
        ([field, label, min, max, fallback]) =>
          `<label>${escapeHTML(label)}<input id="setting-${field}" type="number" min="${min}" max="${max}" required value="${escapeHTML(state.settings[field] ?? fallback)}"></label>`,
      )
      .join('') +
    `<label>全局 worker（重启生效）<input value="${escapeHTML(state.settings.workers || 2)}" disabled></label><label>队列容量（重启生效）<input value="${escapeHTML(state.settings.queue || 32)}" disabled></label><label>每凭证并发<input value="1" disabled></label>`;
  openEditor('settings-dialog');
}

function exportRun() {
  if (!state.run) return;
  const blob = new Blob([JSON.stringify(state.run, null, 2)], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `model-quality-${state.run.id || 'run'}.json`;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 0);
}

async function dispatchClick(event) {
  const button = event.target.closest('button,[data-run]');
  if (!button) return;
  if (button.dataset.close) {
    closeSurface(button.dataset.close);
    if (button.dataset.close === 'release-dialog') { state.reads.preview++; state.releaseBlocked = true; }
    if (button.dataset.close === 'preview-dialog') $('safe-preview').srcdoc = '';
    return;
  }
  if (button.dataset.manualView) {
    setManualView(button.dataset.manualView, true);
    if (button.dataset.manualView === 'history' && state.connected) await loadHistory();
    return;
  }
  if (button.id === 'reading-toggle') {
    setReadingExpanded(!state.readingExpanded);
    button.focus({ preventScroll: true });
    return;
  }
  if (button.dataset.opsView) { setOpsView(button.dataset.opsView); return; }
  if (button.dataset.page) {
    setPage(button.dataset.page);
    return;
  }
  if (button.dataset.run) {
    setPage('manual');
    await loadRun(button.dataset.run);
    return;
  }
  if (button.dataset.expandSample != null) {
    const output = button.closest('.sample').querySelector('.answer-text');
    const expanded = output.classList.toggle('expanded');
    button.textContent = expanded ? '收起长回答' : '展开完整回答';
    button.setAttribute('aria-expanded', String(expanded));
    if (!expanded) button.scrollIntoView({ block: 'nearest' });
    return;
  }
  if (button.dataset.copySample != null) {
    await navigator.clipboard.writeText(state.run.samples[Number(button.dataset.copySample)]?.output || '');
    localFeedback(button, '已复制原文');
    return;
  }
  if (button.dataset.copyModel != null) {
    await navigator.clipboard.writeText(button.dataset.copyModel);
    localFeedback(button, '已复制 ID');
    return;
  }
  if (button.dataset.previewSample != null) {
    const preview = safePreview(
      state.run.samples[Number(button.dataset.previewSample)]?.output || '',
    );
    $('preview-dialog').showModal();
    $('safe-preview').srcdoc = preview;
    return;
  }
  if (button.dataset.evidenceSample != null) {
    const sample = state.run.samples[Number(button.dataset.evidenceSample)];
    $('evidence-content').innerHTML = renderSampleEvidence(sample);
    $('evidence-dialog').showModal();
    return;
  }
  if (button.dataset.reviewSample != null) {
    const index = Number(button.dataset.reviewSample);
    await api('/runs/review', 'POST', {
      id: state.run.id,
      sample_index: index,
      verdict: value(`review-${index}`),
      note: value(`review-note-${index}`),
    });
    await loadRun(state.run.id, false);
    localFeedback(button, '评审已保存');
    return;
  }
  if (button.dataset.ruleEdit) {
    protectEditor(() => editRule(state.rules.find((rule) => rule.id === button.dataset.ruleEdit)));
    return;
  }
  if (button.dataset.ruleRun) {
    const rule = state.rules.find((entry) => entry.id === button.dataset.ruleRun);
    const blocked = rule ? blockedConfig(rule.config || rule) : '规则当前不可用';
    if (blocked) throw new Error(blocked);
    if (!rule?.enabled || ruleHeld(rule))
      throw new Error('检测已停止。请先人工解除凭证暂停，再显式恢复检测规则。');
    const reply = await api('/rules/run', 'POST', { id: button.dataset.ruleRun });
    setPage('manual');
    await loadRun(reply.run_id || reply.id);
    return;
  }
  if (button.dataset.rulePause) {
    const scope = feedbackScope();
    const rule = state.rules.find((entry) => entry.id === button.dataset.rulePause);
    if (!state.connected || !state.targetsFresh) throw new Error('凭证状态尚未确认，请刷新后再操作检测规则。');
    if (ruleHeld(rule)) throw new Error('凭证仍被暂停，不能恢复检测规则。');
    await api('/rules/pause', 'POST', { id: rule.id, revision: rule.revision, paused: rule.enabled });
    if (scope === feedbackScope()) notice(rule.enabled ? '检测规则已暂停。已有凭证暂停仍然生效。' : '检测规则已恢复。');
    await loadOperations();
    return;
  }
  if (button.dataset.ruleDelete) {
    const rule = state.rules.find((entry) => entry.id === button.dataset.ruleDelete);
    actionDialog({ title: '删除检测规则', description: `删除“${rule.name || rule.id}”后无法撤销。已有运行记录保留；持有暂停的规则须先处理暂停归属。`, confirm: '删除规则', danger: true, submit: async () => {
      await api('/rules', 'DELETE', { id: rule.id, revision: rule.revision });
      await loadOperations();
      if (state.page === 'operations') notice(`规则“${rule.name || rule.id}”已删除。`);
    } });
    return;
  }
  if (button.dataset.actionPreview != null) await previewAction(Number(button.dataset.actionPreview));
}

document.addEventListener('click', (event) => {
  const button = event.target.closest(
    '[data-rule-run],[data-rule-pause],[data-rule-delete],[data-action-preview],[data-review-sample]',
  );
  if (button?.disabled) return;
  safely(() => dispatchClick(event), button ? `action:${JSON.stringify(button.dataset)}` : '', button);
});
document.addEventListener('keydown', (event) => {
  if (['Enter', ' '].includes(event.key) && event.target.matches('[data-run]')) { event.preventDefault(); event.target.click(); }
});
$('connection-top').addEventListener('click', () => $('connection-dialog').showModal());
$('connection-open').addEventListener('click', () => $('connection-dialog').showModal());
$('connection-form').addEventListener('submit', (event) => {
  event.preventDefault();
  safely(() => connectManagement(value('management-key')), 'connect', event.submitter);
});
$('disconnect').addEventListener('click', () => disconnect());
$('refresh').addEventListener('click', () => safely(refresh, 'refresh', $('refresh')));
$('target').addEventListener('change', () => renderTarget(true));
$('models').addEventListener('change', () => { renderReasoning(true); renderTarget(); });
$('reasoning').addEventListener('change', () => { renderReasoning(true); renderAdmission(); });
$('question').addEventListener('change', renderQuestion);
$('prompt').addEventListener('input', () => {
  updateQuestionNote();
  renderAdmission();
});
$('rule-question').addEventListener('change', () =>
  $('rule-control-fields').classList.toggle('hidden', value('rule-question') !== 'random-linear'),
);
$('run-form').addEventListener('submit', (event) => {
  event.preventDefault();
  safely(() => submitRun(runConfig()), 'submit-run', $('run-submit'));
});
$('run-cancel').addEventListener('click', () =>
  safely(async () => {
    const id = state.run.id || state.run.run_id;
    const scope = feedbackScope();
    await api('/runs/cancel', 'POST', { id });
    if (id === (state.run?.id || state.run?.run_id)) {
      if (scope === feedbackScope()) notice('已请求取消；后台会关闭对应模型操作。');
      await loadRun(id, false);
    }
    await loadHistory();
  }, 'cancel-run', $('run-cancel')),
);
$('run-rerun').addEventListener('click', () =>
  safely(
    async () => {
      await submitRun(copyRunConfig(state.run.config));
    },
    'submit-run',
    $('run-rerun'),
  ),
);
$('run-export').addEventListener('click', exportRun);
$('history-prev').addEventListener('click', () =>
  safely(async () => {
    state.historyOffset = Math.max(0, state.historyOffset - 10);
    await loadHistory();
  }),
);
$('history-next').addEventListener('click', () =>
  safely(async () => {
    state.historyOffset += 10;
    await loadHistory();
  }),
);
$('rule-new').addEventListener('click', () => protectEditor(() => editRule()));
$('rule-form').addEventListener('submit', (event) => {
  event.preventDefault();
  safely(() => saveEditor(saveRule), 'save-rule', event.submitter);
});
$('release-confirm').addEventListener('click', () =>
  safely(async () => {
    if (state.releaseBlocked || !state.release) return;
    const binding = { ...state.release };
    const sequence = state.reads.preview;
    const current = () => sequence === state.reads.preview && $('release-dialog').open;
    state.releaseBlocked = true;
    releaseFeedback('正在提交人工解除。请等待宿主确认…');
    try {
      await api('/actions/release', 'POST', { ...binding, pause_rule: true });
      if (!current()) { await loadOperations(); return; }
      releaseFeedback('宿主已确认释放此暂停归属。正在核对剩余限制…');
      $('release-confirm').textContent = '暂停归属已释放';
      await reconcileRelease();
    } catch (error) {
      if (isDiscarded(error) || !current()) return;
      const conflict = error.status === 409;
      releaseFeedback(conflict ? `动作版本或归属冲突：${error.message}。当前预览已失效，请刷新后重新核对。` : `解除结果尚未确认：${error.message}。宿主可能已执行解除；请刷新核对，不要重复提交。`, true);
      notice(conflict ? '解除发生版本冲突，请在预览中刷新核对。' : '解除结果尚未确认，请在预览中刷新核对。', true);
    } finally {
      if (current()) $('release-refresh').classList.remove('hidden');
      renderAdmission();
    }
  }, 'release', $('release-confirm')),
);
$('release-refresh').addEventListener('click', () => safely(reconcileRelease, 'release-refresh', $('release-refresh')));
$('settings-open').addEventListener('click', () => protectEditor(() => safely(openSettings, 'settings-read', $('settings-open'))));
$('settings-form').addEventListener('submit', (event) => {
  event.preventDefault();
  safely(
    () => saveEditor(async () => {
      const settings = {
        ...state.settings,
        scheduling_enabled: checked('settings-scheduling'),
        automatic_pause: false,
      };
      delete settings.automatic_recovery;
      delete settings.capabilities;
      for (const [field] of budgetFields) settings[field] = number(`setting-${field}`);
      await api('/settings', 'PUT', settings);
      closeSurface('settings-dialog', true);
      if (state.page === 'operations') notice('全局开关与预算已保存。');
    }),
    'settings-save',
    event.submitter,
  );
});
window.addEventListener('beforeunload', (event) => {
  if (editorDirty()) { event.preventDefault(); event.returnValue = ''; }
});
window.addEventListener('pagehide', () => {
  client.clear();
  loginWorkbench.reset();
  clearTimeout(state.poll);
  stopThemeFollowing();
});
window.addEventListener('pageshow', (event) => {
  if (!event.persisted) return;
  disconnect('', false, { forgetKey: false });
  initializeTheme();
  restoreManagementConnection();
});
document.body.classList.toggle('embedded', window.self !== window.top);
setPage(location.hash.slice(1));
setManualView('results');
setReadingExpanded(false);
setOpsView('attention');
initializeTheme();
$('theme-toggle').addEventListener('change', () => {
  if (document.body.classList.contains('host-themed')) return;
  const selected = value('theme-toggle');
  if (!['light', 'white', 'dark'].includes(selected)) return;
  localTheme = selected;
  document.documentElement.dataset.theme = selected;
});
$('action-form').addEventListener('submit', async (event) => {
  event.preventDefault();
  const request = actionRequest;
  if (!request || state.busy.has('dialog-action')) return;
  if (request.input && !value('action-name')) {
    $('action-error').textContent = '请输入预设名称。';
    $('action-error').classList.remove('hidden');
    $('action-name').setAttribute('aria-invalid', 'true');
    $('action-name').focus();
    return;
  }
  state.busy.add('dialog-action');
  const button = $('action-confirm');
  const label = button.textContent;
  button.style.minWidth = `${button.getBoundingClientRect().width}px`;
  button.textContent = request.input ? '保存中…' : '处理中…';
  button.disabled = true;
  button.setAttribute('aria-busy', 'true');
  try {
    await request.submit(value('action-name'));
    if (actionRequest === request && request.epoch === client.epoch) $('action-dialog').close();
  } catch (error) {
    if (!isDiscarded(error) && actionRequest === request && request.epoch === client.epoch) {
      $('action-error').textContent = error.message || '操作失败，请重试。';
      $('action-error').classList.remove('hidden');
      $('action-error').focus();
    }
  } finally {
    state.busy.delete('dialog-action');
    button.disabled = false;
    button.removeAttribute('aria-busy');
    button.style.removeProperty('min-width');
    button.textContent = label;
  }
});
$('action-dialog').addEventListener('cancel', (event) => { if (state.busy.has('dialog-action')) event.preventDefault(); });
$('action-dialog').addEventListener('close', () => { actionRequest = null; $('action-name').value = ''; });
document.addEventListener('input', (event) => {
  const field = event.target;
  if (field.id && field.hasAttribute('aria-invalid')) {
    field.removeAttribute('aria-invalid');
    const errorID = `${field.id}-error`;
    $(errorID)?.remove();
    field.setAttribute('aria-describedby', (field.getAttribute('aria-describedby') || '').split(' ').filter((id) => id !== errorID).join(' '));
  }
  if (field.closest('.inline-editor')) updateEditorDirty();
});
document.addEventListener('change', (event) => { if (event.target.closest('.inline-editor')) updateEditorDirty(); });
$('samples').addEventListener('change', updatePlan);
for (const id of ['compare-model-a', 'compare-model-b', 'sample-filter']) $(id).addEventListener('change', () => {
  if (value('compare-model-a') === value('compare-model-b')) $('compare-model-b').value = '';
  applyComparison();
});
$('preview-dialog').addEventListener('close', () => { $('safe-preview').srcdoc = ''; });
$('release-dialog').addEventListener('close', () => { state.reads.preview++; state.releaseBlocked = true; });
window.addEventListener('hashchange', () => setPage(location.hash.slice(1)));
renderMetrics({});
renderRules();
renderTargets();
renderActivityState();
renderAdmission();

const phoneLayout = matchMedia('(max-width: 600px)');
const adaptTaskActions = () => { $('task-actions').open = !phoneLayout.matches; };
phoneLayout.addEventListener('change', adaptTaskActions);
adaptTaskActions();
restoreManagementConnection();
$('task-actions').addEventListener('keydown', (event) => {
  if (event.key === 'Escape' && phoneLayout.matches) {
    $('task-actions').open = false;
    $('task-actions').querySelector('summary').focus();
  }
});
document.addEventListener('click', (event) => {
  if (phoneLayout.matches && !event.target.closest('#task-actions')) $('task-actions').open = false;
});
