import { ManagementClient, isDiscarded } from './client.js';
import { createLoginWorkbench } from './login.js';
import {
  active,
  copyRunConfig,
  credentialBlock,
  credentialActions,
  configuredUpstreamActions,
  handlesCredential,
  upstreamConditions,
  date,
  escapeHTML,
  holdsPause,
  items,
  parseModels,
  ruleModelShortcuts,
  splitRuleModels,
  safePreview,
  statusTag,
  targetID,
  renderRunEvidence,
  renderQualityDetail,
  renderManualVerdict,
  renderSampleEvidence,
  renderRuleRows,
  renderScopeDispatch,
  renderScopeSkipDetails,
  dynamicScopeLabel,
  renderQualityRuns,
  renderQualityEmpty,
  ruleState,
  renderReleasePreview,
  questionTitle,
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
  metrics: null,
  runs: [],
  run: null,
  readingRun: null,
  pendingManual: null,
  historyOffset: 0,
  historyTotal: null,
  poll: null,
  pollFailures: 0,
  release: null,
  settings: {},
  scopeBatch: '', scopeRule: '', batchOffset: 0, activityTotal: 0, activityRows: null,
  receiptBatch: '',
  page: 'login',
  manualView: 'results',
  readingExpanded: false,
  releaseBlocked: true,
  releaseAction: null,
  compareRun: null,
  reads: { run: 0, history: 0, activity: 0, operations: 0, refresh: 0, preview: 0, editor: 0 },
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
const manualReceiptStorage = `model-quality:manual-receipt:${managementPrefix}`;
let manualReceiptKey = '';
try { manualReceiptKey = sessionStorage.getItem(manualReceiptStorage) || ''; } catch { /* Use page memory when storage is unavailable. */ }

function rememberManualReceipt(key) {
  manualReceiptKey = key;
  try {
    if (key) sessionStorage.setItem(manualReceiptStorage, key);
    else sessionStorage.removeItem(manualReceiptStorage);
  } catch { /* The receipt still survives a reconnect within this page. */ }
}

const shortcutStorage = `model-quality:rule-model-shortcuts:${managementPrefix}`;
let selectedRuleModels = [], customShortcuts = [], shortcutsLoaded = false, shortcutStorageNote = '';
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
const feedbackScope = () => `${state.page}:${state.page === 'manual' ? state.manualView : ''}`;
function clearNotice() {
  clearTimeout(noticeTimer);
  $('notice').classList.add('hidden');
  $('notice').textContent = '';
  if ($('notice').parentNode !== $('main')) $('main').prepend($('notice'));
}

function notice(message, error = false, before = null) {
  clearNotice();
  if (before) $(before).before($('notice'));
  $('notice').textContent = message;
  $('notice').classList.remove('hidden');
  $('notice').classList.toggle('error', error);
  const dialog = document.querySelector('dialog[open], .inline-editor:not(.hidden)');
  if (error && !before && dialog && dialog.id !== 'release-dialog') {
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
  const fields = [...$(id).querySelectorAll('input,select,textarea')].filter(field => !field.hasAttribute('data-preference')).map((field) => [field.id, ['checkbox','radio'].includes(field.type) ? field.checked : field.value]);
  return JSON.stringify(id === 'rule-dialog' ? [fields, selectedRuleModels] : fields);
}
function editorDirty() { return state.editor && state.editor.snapshot !== editorSnapshot(state.editor.id); }
function updateEditorDirty() {
  if (!state.editor) return;
  const indicator = $(state.editor.id).querySelector('.editor-dirty'), dirty = editorDirty();
  if (indicator.dataset.changed === String(dirty)) return;
  indicator.dataset.changed = String(dirty);
  indicator.textContent = dirty ? '有未保存更改' : state.editor.id === 'rule-dialog' ? '' : '尚未更改';
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
  let failedField;
  try { await action(editor); }
  catch (error) { failedField = error.field; throw error; }
  finally {
    editor.saving = false;
    controls.forEach(([node, disabled]) => { node.disabled = disabled; });
    if (state.editor === editor && failedField) $(failedField).focus();
    renderAdmission();
  }
}

function markFieldError(id, message, focus = true) {
  const field = $(id);
  const errorID = `${id}-error`;
  let feedback = $(errorID);
  if (!feedback) {
    feedback = document.createElement('span');
    feedback.id = errorID;
    feedback.className = 'field-error';
    feedback.setAttribute('role', 'alert');
    (field.closest('.rule-stepper') || field).after(feedback);
  }
  if (feedback.textContent !== message) feedback.textContent = message;
  field.setAttribute('aria-invalid', 'true');
  field.setAttribute('aria-describedby', [...new Set((field.getAttribute('aria-describedby') || '').split(' ').filter(Boolean).concat(errorID))].join(' '));
  if (focus) {
    field.closest('details')?.setAttribute('open', '');
    field.focus();
  }
  return feedback;
}

function fieldError(id, message) {
  markFieldError(id, message);
  const error = new Error(message);
  error.field = id;
  throw error;
}

async function safely(action, key = '', button = null) {
  if (key && state.busy.has(key)) return;
  if (key) state.busy.add(key);
  const scope = feedbackScope();
  if (key) document.querySelectorAll('.dialog-message').forEach((node) => node.remove());
  const originalContent = button ? [...button.childNodes] : [];
  if (key === 'submit-run') renderAdmission();
  const busyLabel = key.includes('save') || key === 'save-rule' ? '保存中…' : key.includes('refresh') ? '读取状态中…' : key === 'connect' ? '连接中…' : '';
  if (button) {
    button.style.minWidth = `${button.getBoundingClientRect().width}px`;
    button.disabled = true;
    button.setAttribute('aria-busy', 'true');
    if (busyLabel) button.textContent = busyLabel;
  }
  try {
    return await action();
  } catch (error) {
    if (!isDiscarded(error) && scope === feedbackScope() && !(key === 'save-rule' && error.field)) notice(error.message || '操作失败，请重试。', true);
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

function updateFreshness() {
  const statuses = Object.values(state.resourceStatus);
  const failed = statuses.includes('error');
  if (!failed && statuses.includes('loading')) return;
  $('data-state').textContent = failed ? '部分数据更新失败，请刷新重试' : `更新于 ${new Date().toLocaleTimeString('zh-CN', { hour12: false })}`;
  $('data-state').classList.toggle('stale', failed);
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
    renderActivityState();
    if (state.page === 'login') loginWorkbench.open({ focus: false });
    if (manualReceiptKey) void checkManualReceipt(manualSelection(), false);
    else schedulePoll();
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
    metrics: null,
    runs: [],
    run: null,
    readingRun: null,
    pendingManual: null,
    pollFailures: 0,
    release: null,
    settings: {}, scopeBatch: '', scopeRule: '', batchOffset: 0, activityTotal: 0, activityRows: null,
    historyOffset: 0,
    historyTotal: null,
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
  selectedRuleModels = [];
  $('rule-model-selection').replaceChildren();
  $('ops-batch-bar').hidden = true;
  clearScopeReceipt();
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
  $('rule-search').value = '';
  $('rule-filter').value = 'all';
  setManualView('setup');
  $('data-state').textContent = failed ? '管理连接失效，请重新连接' : '管理接口未连接';
  $('data-state').classList.add('stale');
  renderRun();
  $('operations-history').replaceChildren();
  renderSelectors();
  renderHistory();
  renderRules();
  renderMetrics();
  renderActivityState();
  renderAdmission();
  if (!forgotten) notice('已断开当前连接，但浏览器未允许清除已保存的管理密钥。请清除此站点的本地存储。', true);
  else if (message) notice(message, failed);
  else clearNotice();
}

const workspaces = {
  login: ['账号管理', $('page-description').textContent],
  manual: ['智商测试', '选择凭证与模型，测试并比较回答。'],
  operations: ['质量运维', '管理检测规则，查看模型质量与运行记录。'],
};

function setPage(page) {
  if (page !== state.page) clearNotice();
  state.page = ['login', 'manual', 'operations'].includes(page) ? page : 'login';
  if (state.page === 'login') loginWorkbench.open({ focus: false });
  else loginWorkbench.close({ focus: false });
  $('page-manual').classList.toggle('hidden', state.page !== 'manual');
  $('page-operations').classList.toggle('hidden', state.page !== 'operations');
  $('page-login').classList.toggle('hidden', state.page !== 'login');
  const [name, description] = workspaces[state.page];
  $('page-crumb').textContent = name;
  $('page-description').textContent = description;
  document.querySelectorAll('[data-page]').forEach((button) => {
    button.classList.toggle('active', button.dataset.page === state.page);
    button.setAttribute('aria-current', button.dataset.page === state.page ? 'page' : 'false');
  });
  history.replaceState(null, '', `#${state.page}`);
  scheduleWorkspaceLayout();
}

let workspaceLayoutFrame = 0;
function scheduleWorkspaceLayout() {
  cancelAnimationFrame(workspaceLayoutFrame);
  workspaceLayoutFrame = requestAnimationFrame(() => {
    if (!['operations', 'manual'].includes(state.page)) return;
    const manual = state.page === 'manual';
    const workspace = document.querySelector(manual ? '.manual-workbench' : '.quality-workspace');
    // Measure the document offset so scrolling never changes the panel height.
    const top = workspace.getBoundingClientRect().top + window.scrollY;
    const compactManual = manual && window.innerWidth < 960;
    const minimum = manual ? (compactManual ? 380 : 520) : 460;
    const bottomSpace = compactManual ? 16 : manual ? 32 : 24;
    workspace.style.setProperty(manual ? '--manual-panel-height' : '--quality-panel-height', `${Math.max(minimum, Math.floor(window.innerHeight - top - bottomSpace))}px`);
  });
}

function setManualPane(view) {
  $('page-manual').dataset.pane = view;
  for (const button of document.querySelectorAll('.manual-pane-switch button')) {
    button.setAttribute('aria-pressed', String(button.dataset.manualPane === view));
  }
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
  $('manual-result-footer').classList.toggle('hidden', state.manualView !== 'results');
  $('manual-history-footer').classList.toggle('hidden', state.manualView !== 'history');
  document.querySelector('.answer-workspace')?.classList.remove('hidden');
  setManualPane(view);
  document.querySelectorAll('.view-nav [data-manual-view]').forEach((button) => {
    const selected = button.dataset.manualView === state.manualView;
    button.classList.toggle('active', selected);
    button.setAttribute('aria-current', selected ? 'page' : 'false');
    button.setAttribute('aria-controls', `manual-${button.dataset.manualView}`);
  });
  $('page-manual').dataset.view = state.manualView;
  $('manual-title').textContent = '智商测试';
  scheduleWorkspaceLayout();
  if (focus) {
    const section = $(showConfiguration ? 'manual-setup' : `manual-${state.manualView}`);
    section.tabIndex = -1;
    section.focus({ preventScroll: true });
    section.scrollIntoView({ block: 'nearest', behavior: 'instant' });
  }
}

function setReadingExpanded(expanded) {
  state.readingExpanded = expanded;
  if (expanded && $('page-manual').dataset.pane === 'setup') setManualPane(state.manualView);
  $('manual-setup').classList.toggle('hidden', expanded);
  $('page-manual').dataset.readingExpanded = String(expanded);
  const toggle = $('reading-toggle');
  if (!toggle) return;
  toggle.textContent = expanded ? '返回配置' : '展开阅读';
  toggle.setAttribute('aria-pressed', String(expanded));
  toggle.setAttribute('aria-controls', 'manual-setup');
  scheduleWorkspaceLayout();
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
  $(id).showModal();
  $(id).querySelector('.rule-form-scroll').scrollTop = 0;
  $('rule-name').focus({ preventScroll: true });
  $(id).scrollTop = 0;
}

function closeSurface(id, saved = false) {
  const node = $(id);
  if (state.editor?.id === id) {
    if (!saved && state.editor.saving) return;
    if (!saved && editorDirty()) { protectEditor(() => closeSurface(id, true)); return; }
    const opener = state.editor.opener?.isConnected ? state.editor.opener : $('rule-new');
    state.editor = null;
    node.close();
    // Nested discard confirmation remains modal until its own submit completes.
    if (!$('action-dialog').open) opener.focus({ preventScroll: true });
    else $('action-dialog').addEventListener('close', () => opener.isConnected && opener.focus({ preventScroll: true }), { once: true });
    return;
  }
  if (node.tagName === 'DIALOG' && (id !== 'action-dialog' || !state.busy.has('dialog-action'))) node.close();
}
$('rule-dialog').addEventListener('cancel', event => { event.preventDefault(); closeSurface('rule-dialog'); });

function targetOptions() {
  const seen = new Set(), duplicates = new Set();
  for (const target of state.targets) {
    if (!target.email) continue;
    if (seen.has(target.email)) duplicates.add(target.email);
    seen.add(target.email);
  }
  return '<option value="">选择凭证</option>' + state.targets.map(target => {
    const label = target.email || `${target.provider || 'unknown'} · ${target.auth_index || targetID(target)}`;
    const suffix = duplicates.has(target.email) ? ` · ${target.auth_index || targetID(target)}` : '';
    return `<option value="${escapeHTML(targetID(target))}">${escapeHTML(label + suffix)}${target.quality_paused ? ' · 质量暂停' : ''}${target.manual_disabled ? ' · 人工禁用' : ''}</option>`;
  }).join('');
}

function credentialName(id) {
  return findTarget(id)?.email || id;
}

function questionOptions() {
  return (
    state.questions
      .map(
        (question) =>
          `<option value="${escapeHTML(question.id)}">${escapeHTML(questionTitle(question))}</option>`,
      )
      .join('') +
    (state.questions.some((question) => question.id === 'custom')
      ? ''
      : '<option value="custom">自定义题目</option>')
  );
}

function renderSelectors() {
  renderTargetSelectors();
  renderQuestionSelectors();
}

function renderTargetSelectors() {
  const options = targetOptions();
  for (const id of state.editor ? ['target'] : ['target', 'rule-target', 'rule-judge-target', 'rule-control-target']) {
    const previous = $(id).value;
    $(id).innerHTML = options;
    if (findTarget(previous)) $(id).value = previous;
  }
  renderTarget();
  if (state.editor) updateRuleFields();
}

function renderQuestionSelectors() {
  const previousQuestion = value('question');
  const previousPrompt = $('prompt').value;
  $('question-read-note').hidden = state.resourceStatus.questions !== 'error';
  const curated = ['logic-candy', 'visual-pelican-html', 'custom'];
  const questions = curated.map((id) => state.questions.find((question) => question.id === id)).filter(Boolean);
  $('question').innerHTML = questions.length
    ? questions.map((question) => `<option value="${escapeHTML(question.id)}">${escapeHTML(questionTitle(question))}</option>`).join('')
    : '<option value="">连接后加载题目</option>';
  if (questions.some((question) => question.id === previousQuestion)) $('question').value = previousQuestion;
  if (!state.editor) {
    const ruleQuestion = value('rule-question');
    $('rule-question').innerHTML = questionOptions();
    if ([...$('rule-question').options].some((option) => option.value === ruleQuestion)) $('rule-question').value = ruleQuestion;
  }
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
    $('target-summary').textContent = resourceMessage('targets', '测试凭证') || (state.targets.length ? '选择要测试的凭证。' : '尚无可用凭证，请先在 CPA 中添加凭证。');
  } else {
    const reason = !state.targetsFresh ? '凭证目录读取失败，当前状态未确认。请刷新重试。' : credentialBlock({ auth_id: targetID(target) }, state.targets);
    $('target').title = target.email || targetID(target);
    patchView($('target-summary'), reason ? `<span class="error-note">${escapeHTML(reason)}</span>` : '<span>凭证可用</span>');
  }
  renderReasoning(selectModel);
  renderAdmission();
}

const questionTextEncoder = new TextEncoder();
function customQuestionTextError(field) {
  const standard = field.id === 'rule-review-standard';
  // Match Go's strings.TrimSpace emptiness check and count the original UTF-8 text.
  if (!standard && !/[^\p{White_Space}]/u.test(field.value)) return '请填写题目内容。';
  const limit = standard ? 4096 : 16384;
  const excess = questionTextEncoder.encode(field.value).length - limit;
  return excess > 0 ? `至少删减 ${excess.toLocaleString('zh-CN')} 字节（UTF-8 上限 ${limit.toLocaleString('zh-CN')}）` : '';
}

function updateQuestionTextValidity(field, custom) {
  const message = custom ? customQuestionTextError(field) : '';
  field.setCustomValidity(message);
  const marked = field.hasAttribute('aria-invalid');
  if (message && (field.value || marked)) {
    const error = markFieldError(field.id, message, false);
    if (!marked && document.activeElement === field) error.scrollIntoView({ block: 'nearest', container: 'nearest' });
  } else clearFieldError(field);
}

function readCustomQuestionText(id) {
  const field = $(id), message = customQuestionTextError(field);
  if (message) fieldError(id, message);
  return field.value;
}

function updateQuestionNote() {
  const question = state.questions.find((entry) => entry.id === value('question'));
  const selected = questionTestSelection(question, $('prompt').value);
  const changed = !!question && question.id !== 'custom' && selected.question_id === 'custom';
  $('question-version').textContent = changed ? '已修改' : '';
  const note = $('prompt-note');
  note.hidden = !changed;
  note.textContent = changed ? '按自定义题目测试，不沿用参考答案。' : '';
  updateQuestionTextValidity($('prompt'), selected.question_id === 'custom');
  renderAdmission();
}

function renderQuestion() {
  const question = state.questions.find((entry) => entry.id === value('question'));
  $('prompt').readOnly = false;
  $('prompt').value = question?.prompt || '';
  updateQuestionNote();
}

function manualRunConfig() {
  const target = findTarget(value('target'));
  if (!target) fieldError('target', '请选择要测试的凭证。');
  const models = parseModels(value('models'));
  if (models.length !== 1 || !availableModels(target).includes(models[0])) fieldError('models', '请选择此凭证目录中的可用模型。');
  if ($('reasoning').dataset.unavailable === 'true') fieldError('reasoning', '请重新选择可用的思考强度。');
  const samples = number('samples');
  if (!Number.isInteger(samples) || samples < 1 || samples > 8) fieldError('samples', '回答数量须为 1–8 的整数。');
  const question = state.questions.find((entry) => entry.id === value('question'));
  const selection = questionTestSelection(question, $('prompt').value);
  if (selection.question_id === 'custom') readCustomQuestionText('prompt');
  const config = {
    auth_id: targetID(target), provider: target.provider, models, samples,
    ...selection,
    parameters: { stream: true },
    judge: { mode: 'manual' },
  };
  if (value('reasoning')) config.parameters.reasoning_effort = value('reasoning');
  return config;
}

function runConfig(prefix = '') {
  if (!prefix) return manualRunConfig();
  const get = (name) => value(`${prefix}${name}`);
  const scope = $('rule-form').elements.namedItem('rule-scope').value;
  const all = !!dynamicScopeLabel(scope);
  const target = all ? null : findTarget(get('target'));
  if (!all && !target) fieldError(`${prefix}target`, '请选择目标凭证。');
  const models = draftRuleModels();
  const problem = ruleModelsProblem(models);
  if (problem) fieldError(`${prefix}models`, problem);
  const plan = ruleSampling(models);
  if (plan.over) fieldError(`${prefix}samples`, `超过每凭证样本限额 ${plan.limit}。请减少模型或样本数。`);
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
    target_scope: scope,
    ...(!all ? {auth_id: targetID(target), provider: target.provider} : {}),
    models,
    question_id: get('question'),
    samples: number(`${prefix}samples`),
    parameters: {
      stream: state.rules.find(r => r.id === value('rule-id'))?.parameters?.stream ?? true,
    },
    judge,
  };
  if (!all && config.question_id === 'random-linear' && get('control-target')) {
    const control = findTarget(get('control-target'));
    if (!control || targetID(control) === targetID(target))
      fieldError(`${prefix}control-target`, '随机题通过对照必须选择另一个凭证。');
    config.control_auth_id = targetID(control);
    config.control_provider = control.provider;
  }
  const effort = $('rule-reasoning').querySelector('input:checked')?.value;
  if (effort) config.parameters.reasoning_effort = effort;
  if (config.question_id === 'custom') {
    config.prompt = readCustomQuestionText(`${prefix}prompt`);
    config.review_standard = readCustomQuestionText(`${prefix}review-standard`);
  }
  return config;
}

function blockedConfig(config) {
  if (!state.connected) return '请先连接管理接口。';
  if (!state.targetsFresh) return '凭证状态尚未确认，请刷新后再执行。';
  return credentialBlock(config, state.targets);
}

function blockedManualConfig(config) {
  return blockedConfig(config) || (state.resourceStatus.questions === 'error' ? '题库读取失败，请刷新后再测试。' : '');
}

function renderAdmission() {
  const config = {
    auth_id: value('target'),
    judge: { mode: 'manual' },
  };
  const blocked = blockedManualConfig(config) || (config.auth_id && !availableModels(findTarget(config.auth_id)).includes(value('models')) ? '请先选择可用的测试模型。' : '') || ($('reasoning').dataset.unavailable === 'true' ? '请重新选择可用的思考强度。' : '');
  const testing = !!state.pendingManual || state.busy.has('submit-run');
  const phase = state.pendingManual?.status;
  const retrySubmission = state.connected && !testing && !!manualReceiptKey;
  $('manual-config-summary').textContent = phase === 'checking_receipt' ? '正在查询上次提交' : retrySubmission ? '未查到上次任务' : `${value('samples')} 份回答`;
  $('run-submit-label').textContent = testing ? ({ queued: '排队中', running: '测试中', cancelling: '停止中', checking_receipt: '核对中' }[phase] || '提交中') : retrySubmission ? '重新提交' : '开始测试';
  $('run-submit').setAttribute('aria-busy', String(testing));
  $('run-submit').disabled = testing || !config.auth_id || !!blocked || !$('prompt').validity.valid;
  $('run-submit').title = state.connected ? blocked || $('prompt').validationMessage : '';
  $('release-confirm').disabled = state.releaseBlocked || state.busy.has('release') || !state.connected;
  for (const id of [
    'rule-new',
    'ops-all-rules',
    'run-export',
    'run-cancel',
    'refresh',
  ])
    $(id).disabled = !state.connected || $(id).getAttribute('aria-busy') === 'true';
  if (state.run) {
    const reason = blockedManualConfig(state.run.config || {});
    $('run-rerun').disabled = !!reason || testing;
    $('run-rerun').title = reason;
  }
}

function newKey() {
  return globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function manualSelection() {
  return { sequence: state.reads.run, scope: feedbackScope() };
}

function trackManualRun(run, selection, focus) {
  state.pollFailures = 0;
  state.pendingManual = active(run) ? { id: run.id, status: run.status } : null;
  if (!state.pendingManual) rememberManualReceipt('');
  if (selection.sequence === state.reads.run) {
    ++state.reads.run;
    state.run = run;
    renderRun();
    if (focus && state.page === 'manual' && selection.scope === feedbackScope()) {
      clearNotice();
      setManualView('results', true);
    }
  }
  renderAdmission();
  schedulePoll(0);
}

// Only an explicit submit may send a POST. A missing receipt can still be
// racing admission, so even that retry must reuse the original key.
async function checkManualReceipt(selection, focus) {
  const key = manualReceiptKey;
  const pending = { status: 'checking_receipt', selection, focus };
  state.pendingManual = pending;
  renderAdmission();
  try {
    const reply = await api(`/runs/detail?idempotency_key=${encodeURIComponent(key)}`);
    if (state.pendingManual !== pending) return;
    const run = reply.run || reply;
    if (!run?.id || run.source !== 'manual_test' || run.config?.idempotency_key !== key) throw new Error('任务回执尚未确认。');
    trackManualRun(run, selection, focus);
    await Promise.allSettled([loadHistory()]);
  } catch (error) {
    if (state.pendingManual !== pending || isDiscarded(error)) return;
    if (error.status === 404 || error.status === 410) {
      state.pendingManual = null;
      state.pollFailures = 0;
      if (error.status === 410) rememberManualReceipt('');
      renderAdmission();
      schedulePoll();
      return error.status === 404 ? 'missing' : 'removed';
    }
    state.pollFailures = Math.min(state.pollFailures + 1, 4);
    schedulePoll();
  }
}

async function submitRun(config) {
  if (state.pendingManual) return;
  const selection = manualSelection();
  const retry = !!manualReceiptKey;
  if (retry && await checkManualReceipt(selection, true) !== 'missing') return;
  const blocked = blockedManualConfig(config);
  if (blocked) throw new Error(blocked);
  const key = manualReceiptKey || newKey();
  rememberManualReceipt(key);
  let id;
  try {
    const reply = await api('/runs', 'POST', { ...config, idempotency_key: key });
    id = reply.run_id || reply.id;
  } catch (error) {
    if (isDiscarded(error)) return;
    if (!error.outcomeUnknown && error.status < 500 && error.status !== 408 && !(retry && error.status === 409)) {
      if (!retry || error.status === 410) rememberManualReceipt('');
      throw error;
    }
  }
  if (!id) {
    await checkManualReceipt(selection, true);
    return;
  }
  trackManualRun({ id, source: 'manual_test', status: 'queued', config, created_at: new Date().toISOString() }, selection, true);
  await loadHistory();
}

function updateRunProgress(run) {
  const id = run.id || run.run_id;
  if (id === state.pendingManual?.id) {
    state.pendingManual = active(run) ? { id, status: run.status } : null;
    if (!state.pendingManual) rememberManualReceipt('');
  }
  const row = state.runs.findIndex(item => (item.id || item.run_id) === id);
  if (row >= 0) {
    state.runs[row] = run;
    renderHistory();
  }
  renderAdmission();
}

async function loadRun(id, show = true) {
  if (!id) throw new Error('接口未返回任务 ID。');
  const sequence = ++state.reads.run;
  state.readingRun = { id, sequence };
  try {
    const reply = await api(`/runs/detail?id=${encodeURIComponent(id)}`);
    if (sequence !== state.reads.run) return;
    state.run = reply.run || reply;
    updateRunProgress(state.run);
    state.pollFailures = 0;
    renderRun();
    $('run-feedback').classList.add('hidden');
    if (show) {
      if (state.page === 'manual') clearNotice();
      setManualView('results', state.page === 'manual');
    }
  } finally {
    if (state.readingRun?.sequence === sequence) state.readingRun = null;
    if (sequence === state.reads.run) schedulePoll();
  }
}

function schedulePoll(delay = Math.min(10000, 1200 * 2 ** state.pollFailures)) {
  clearTimeout(state.poll);
  state.poll = null;
  if (!state.connected) return;
  const pending = state.pendingManual;
  if (pending?.status === 'checking_receipt') {
    state.poll = setTimeout(() => {
      if (state.pendingManual === pending) void checkManualReceipt(pending.selection, pending.focus);
    }, delay);
    return;
  }
  const selected = state.run?.id || state.run?.run_id;
  const ids = [...new Set([active(state.run) && selected, state.pendingManual?.id].filter(Boolean))];
  if (!ids.length) return;
  const epoch = client.epoch;
  const timer = state.poll = setTimeout(async () => {
    const current = () => epoch === client.epoch && timer === state.poll;
    if (!current()) return;
    const results = await Promise.allSettled(ids.map(id => api(`/runs/detail?id=${encodeURIComponent(id)}`)));
    if (!current()) return;
    let finished = false, retry = false;
    for (const [index, result] of results.entries()) {
      const id = ids[index];
      if (result.status === 'rejected') {
        const error = result.reason;
        if (isDiscarded(error)) continue;
        const missing = error.code === 'not_found' || error.status === 404;
        if (missing && id === state.pendingManual?.id) {
          state.pendingManual = null;
          rememberManualReceipt('');
        }
        if (missing && id === selected) {
          state.run = { ...state.run, status: 'unavailable' };
          renderRun();
        }
        if (id === selected) {
          $('run-feedback').textContent = missing ? '这条测试记录已不可用。' : `进度查询失败：${error.message}。会自动重试查询。`;
          $('run-feedback').classList.remove('hidden');
        }
        retry ||= !missing;
        finished ||= missing;
        continue;
      }
      const run = result.value.run || result.value;
      updateRunProgress(run);
      if (id === selected) {
        state.run = run;
        renderRun();
        $('run-feedback').classList.add('hidden');
      }
      finished ||= !active(run);
    }
    state.pollFailures = retry ? Math.min(state.pollFailures + 1, 4) : 0;
    renderAdmission();
    schedulePoll();
    if (finished) await Promise.allSettled([loadHistory(), loadOperations()]);
  }, delay);
}

const detailDialog = $('quality-detail');
let qualityDetail = null;

function openQualityDetail(id) {
  if (!id) throw new Error('接口未返回任务 ID。');
  clearTimeout(qualityDetail?.timer);
  qualityDetail = { id, run: null, read: 0, timer: null };
  $('quality-detail-content').replaceChildren();
  detailDialog.querySelector('footer').hidden = true;
  detailDialog.querySelector('.dialog-message')?.remove();
  $('quality-detail-message').textContent = '正在读取检测记录…';
  $('quality-detail-retry').hidden = true;
  detailDialog.showModal();
  detailDialog.querySelector('.quality-detail-scroll').scrollTop = 0;
  return readQualityDetail(qualityDetail);
}

async function readQualityDetail(detail) {
  if (!detail || detail !== qualityDetail || !detailDialog.open) return;
  clearTimeout(detail.timer);
  const sequence = ++detail.read;
  const current = () => detailDialog.open && detail === qualityDetail && sequence === detail.read;
  try {
    const reply = await api(`/runs/detail?id=${encodeURIComponent(detail.id)}`);
    if (!current()) return;
    const run = reply.run || reply;
    patchView($('quality-detail-content'), renderQualityDetail(run, state.rules, credentialName));
    detail.run = run;
    $('quality-detail-message').textContent = '';
    $('quality-detail-retry').hidden = true;
    $('quality-detail-cancel').hidden = !active(run);
    detailDialog.querySelector('footer').hidden = false;
    if (active(run) || ['disable_pending', 'restore_pending', 'delete_pending'].includes(run.action_status)) {
      detail.timer = setTimeout(() => readQualityDetail(detail), 1500);
    }
    const row = state.activityRows?.findIndex(item => (item.id || item.run_id) === detail.id);
    if (row >= 0) {
      state.activityRows[row] = run;
      patchView($('operations-history'), renderQualityRuns(state.activityRows, state.rules, credentialName));
    }
  } catch (error) {
    if (!current() || isDiscarded(error)) return;
    $('quality-detail-message').textContent = error.status === 404 ? '这条检测记录已不可用。' : `检测记录读取失败：${error.message}`;
    $('quality-detail-retry').hidden = false;
    if (error.status === 404) {
      detail.run = null;
      $('quality-detail-content').replaceChildren();
      detailDialog.querySelector('footer').hidden = true;
    }
  }
}

detailDialog.addEventListener('close', () => {
  if (detailDialog.open) return;
  clearTimeout(qualityDetail?.timer);
  qualityDetail = null;
  $('quality-detail-content').replaceChildren();
});
$('quality-detail-retry').addEventListener('click', () => readQualityDetail(qualityDetail));
$('quality-detail-export').addEventListener('click', () => exportRun(qualityDetail?.run));
$('quality-detail-cancel').addEventListener('click', event => {
  const detail = qualityDetail;
  if (!active(detail?.run)) return;
  safely(async () => {
    await api('/runs/cancel', 'POST', { id: detail.id });
    await readQualityDetail(detail);
  }, `cancel:${detail.id}`, event.currentTarget);
});

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
  if (!run) {
    for (const id of ['run-content', 'task-actions', 'run-feedback', 'run-rerun', 'run-export', 'run-cancel']) $(id).classList.add('hidden');
    for (const id of ['run-frozen-data', 'sample-list', 'run-meta', 'task-indicator']) $(id).replaceChildren();
    $('run-empty').classList.remove('hidden');
    $('manual-result-summary').textContent = '等待开始测试';
    state.compareRun = null;
    renderAdmission();
    return;
  }
  $('run-empty').classList.add('hidden');
  $('run-content').classList.remove('hidden');
  $('task-actions').classList.remove('hidden');
  const repeatable = run.source === 'manual_test' && run.config?.models?.length === 1 && run.config?.judge?.mode === 'manual' && !run.config?.control_auth_id;
  $('run-rerun').classList.toggle('hidden', active(run) || !repeatable);
  $('run-export').classList.remove('hidden');
  $('run-cancel').classList.toggle('hidden', !active(run));
  const rendered = renderRunEvidence(run, credentialName);
  patchView($('run-meta'), rendered.metaHTML);
  patchView($('run-frozen-data'), `<div class="run-identifier">任务 ${escapeHTML(run.id || run.run_id)}</div>${rendered.conditionsHTML}`);
  const progress = Math.min(100, Math.round(rendered.completed / Math.max(1, rendered.planned) * 100));
  $('run-progress').style.width = `${progress}%`;
  $('run-progress').parentElement.setAttribute('aria-valuenow', String(progress));
  $('task-indicator').textContent = active(run) ? '执行中' : '已结束';
  $('manual-result-summary').textContent = `${rendered.completed} / ${rendered.planned} 份${run.source === 'manual_test' ? '回答' : '样本'}已结束`;
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
    setRunDetails(false);
    $('run-feedback').classList.add('hidden');
  }
  applyComparison();
  renderAdmission();
}

function setRunDetails(open) {
  $('run-frozen').setAttribute('aria-expanded', String(open));
  $('run-frozen-data').hidden = !open;
}

$('run-frozen').addEventListener('click', () => setRunDetails($('run-frozen-data').hidden));

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
  const message = resourceMessage('activity', '规则运行记录');
  if (message && !$('operations-history').querySelector('.history-row, .quality-run-table')) patchView($('operations-history'), renderQualityEmpty(message));
  for (const [id, resource, label] of [['rules-read-note', 'rules', '规则'], ['activity-read-note', 'activity', '运行记录']]) {
    $(id).hidden = state.resourceStatus[resource] !== 'error';
    $(id).textContent = `${label}读取失败，当前内容可能已过期。请刷新重试。`;
  }
  $('ops-scope-label').textContent = state.scopeRule ? state.rules.find(r => r.id === state.scopeRule)?.name || '已删除规则的运行记录' : '全部规则';
  $('ops-scope-clear').hidden = !state.scopeRule && !state.scopeBatch;
  $('ops-all-rules').setAttribute('aria-pressed', String(!state.scopeRule && !state.scopeBatch));
  $('ops-record-count').textContent = state.resourceStatus.activity === 'ready' ? `共 ${state.activityTotal} 条` : '';
  $('ops-history-clear').disabled = !state.connected || state.resourceStatus.activity !== 'ready' || (!state.activityTotal && !state.scopeRule && !state.scopeBatch);
  if (state.resourceStatus.activity !== 'ready') $('ops-batch-prev').disabled = $('ops-batch-next').disabled = true;
}

function renderHistory() {
  $('history-total').textContent = state.historyTotal == null ? '任务数未确认' : `${state.historyTotal} 个任务`;
  $('history-count').textContent = state.historyTotal || '';
  $('history-read-note').hidden = state.resourceStatus.history !== 'error' || !state.runs.length;
  patchView($('history-list'), state.runs.length
    ? state.runs
        .map(run => {
          const id = escapeHTML(run.id || run.run_id);
          const title = escapeHTML(questionTitle(run.config?.question || state.questions.find(question => question.id === run.config?.question_id), run.config?.question_id));
          return `<div class="history-row" data-key="run:${id}">
            <button type="button" class="history-open" data-run="${id}">
              <span class="history-primary"><span class="history-title">${title}</span>
              <span class="history-secondary"><span>${escapeHTML(run.config?.models?.join(' / ') || '模型未记录')}</span><span>${escapeHTML((run.config?.models?.length || 1) * (run.config?.samples || 1))} 份回答</span></span>
              <span class="history-time"><time>${escapeHTML(date(run.created_at))}</time><span class="history-credential">${escapeHTML(run.credential_email || credentialName(run.config?.auth_id) || '凭证未记录')}</span></span></span>
              <span class="history-trailing">${renderManualVerdict(run)}${statusTag(run.status)}</span>
            </button>
            <button type="button" class="history-delete" data-delete-run="${id}" aria-label="删除测试记录：${title}" title="${active(run) ? '测试结束后可删除' : '删除记录'}" ${active(run) ? 'disabled' : ''}>
              <svg class="control-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M3 6h18M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2M19 6v14a1 1 0 0 1-1 1H6a1 1 0 0 1-1-1V6M10 10v7m4-7v7"/></svg>
            </button></div>`;
        })
        .join('')
    : `<div class="empty-line calm-empty"><strong>${escapeHTML(resourceMessage('history', '测试记录') || '还没有测试记录')}</strong>${state.resourceStatus.history === 'ready' ? '<span>完成一次测试后，可在这里查看回答、执行详情与历史条件。</span>' : ''}</div>`);
  $('history-page').textContent = `第 ${Math.floor(state.historyOffset / 10) + 1} 页`;
  $('history-prev').disabled = state.historyOffset === 0;
  $('history-next').disabled = state.historyOffset + 10 >= state.historyTotal;
  $('history-pagination').hidden = state.historyTotal <= 10 && state.historyOffset === 0;
}

async function loadHistory(offset = state.historyOffset) {
  const sequence = ++state.reads.history;
  if (!state.resourceStatus.history) state.resourceStatus.history = 'loading';
  if (!state.runs.length) renderHistory();
  let result;
  try {
    result = await api(`/runs?source=manual_test&offset=${offset}&limit=10&view=summary`);
  } catch (error) {
    if (sequence === state.reads.history && !isDiscarded(error)) {
      state.resourceStatus.history = 'error';
      renderHistory();
      updateFreshness();
      return false;
    }
    return;
  }
  if (sequence !== state.reads.history) return;
  const rows = items(result), total = result.total ?? rows.length;
  const lastOffset = Math.max(0, Math.floor((total - 1) / 10) * 10);
  if (offset > lastOffset) return loadHistory(lastOffset);
  state.resourceStatus.history = 'ready';
  state.historyOffset = offset;
  state.runs = rows;
  state.historyTotal = total;
  renderHistory();
  updateFreshness();
  return true;
}

async function cleanupRequest(path, body) {
  const confirmation = actionRequest;
  let receipt = null;
  try {
    const result = await api(path, 'DELETE', body);
    if (path === '/rules/run-history') {
      if ([result?.deleted, result?.kept].every(count => Number.isSafeInteger(count) && count >= 0)) receipt = result;
    } else if (result?.ok === true) receipt = result;
  } catch (error) {
    if (isDiscarded(error) || (!error.outcomeUnknown && error.status >= 400 && error.status < 500 && error.status !== 408)) throw error;
  }
  // A second bulk DELETE could remove runs that completed after the original request.
  confirmation.cleanupSent = true;
  return receipt;
}

async function refreshCleanupLists(epoch) {
  try { await loadOperations(); }
  catch (error) { if (isDiscarded(error)) throw error; }
  return client.epoch === epoch;
}

function focusAfterCleanup(id) {
  $('action-dialog').addEventListener('close', () => {
    if ($(id).getClientRects().length) $(id).focus({ preventScroll: true });
  }, { once: true });
}

function confirmHistoryDelete(id) {
  const run = state.runs.find(item => (item.id || item.run_id) === id);
  if (!run || active(run)) return;
  actionDialog({
    title: '删除测试记录',
    description: `删除「${questionTitle(run.config?.question, run.config?.question_id)}」（${date(run.created_at)}）及本次回答？任务 ${id}，凭证 ${run.credential_email || credentialName(run.config?.auth_id) || '未记录'}。`,
    confirm: '删除', danger: true,
    submit: async () => {
      const epoch = client.epoch;
      const receipt = await cleanupRequest('/runs', { id });
      let removed = !!receipt, checked = '';
      if (!receipt) {
        try {
          const result = await api(`/runs/detail?id=${encodeURIComponent(id)}`);
          if ((result?.run || result)?.id === id) checked = '当前仍可读取此记录。';
        } catch (error) {
          if (isDiscarded(error)) throw error;
          removed = error.status === 404 && error.code === 'not_found';
          if (removed) checked = '已按任务 ID 核对，当前查不到此记录。';
        }
      }
      if (removed) {
        if (state.readingRun?.id === id && state.readingRun.sequence === state.reads.run) ++state.reads.run;
        if ((state.run?.id || state.run?.run_id) === id) { state.run = null; renderRun(); }
        if (state.pendingManual?.id === id) { state.pendingManual = null; rememberManualReceipt(''); }
        schedulePoll();
        const rows = state.runs.filter(item => (item.id || item.run_id) !== id);
        if (rows.length !== state.runs.length) state.historyTotal = Math.max(0, state.historyTotal - 1);
        state.runs = rows;
        renderHistory();
        renderAdmission();
      }
      const refreshed = await loadHistory();
      if (epoch !== client.epoch || state.page !== 'manual') return;
      focusAfterCleanup('manual-history');
      if (!receipt) notice(`删除回执未确认。${checked || '任务状态仍待读取。'}${refreshed === false ? '列表读取失败，请刷新。' : ''}`, true, 'manual-history');
      else if (refreshed === false) notice('记录已删除，列表刷新失败，请刷新重试。', true, 'manual-history');
    },
  });
}

function renderRules() {
  const query = value('rule-search').toLocaleLowerCase(), filter = value('rule-filter');
  const filtered = state.rules.filter(rule => {
    const config = rule.config || rule;
    return (filter === 'all' || ruleState(rule, state.targets) === filter) && [rule.name, rule.id, config.auth_id, credentialName(config.auth_id), ...(config.models || []), dynamicScopeLabel(rule.target_scope)].join(' ').toLocaleLowerCase().includes(query);
  });
  const message = !state.rules.length && resourceMessage('rules', '检测规则');
  const expandedSkips = new Set([...$('rules-list').querySelectorAll('details[data-skip-rule][open]')].map(node => node.dataset.skipRule));
  patchView($('rules-list'), message ? renderQualityEmpty(message) : state.rules.length && !filtered.length ? renderQualityEmpty('没有匹配的规则', '调整搜索内容或状态筛选。', '<button class="text-button" data-rule-clear>清除筛选</button>') : renderRuleRows(filtered, state.targets, { ...state, expandedSkips }));
  $('rules-list').querySelectorAll('[data-rule-new]').forEach(button => { button.disabled = !state.connected; });
  $('rule-count').textContent = state.resourceStatus.rules === 'ready' || state.rules.length ? String(state.rules.length) : '—';
  $('ops-all-count').textContent = $('rule-count').textContent;
  $('rule-summary').textContent = state.resourceStatus.rules === 'ready' || state.rules.length ? `显示 ${filtered.length} / ${state.rules.length} 条规则` : '尚未读取检测规则';
  $('rule-timed-count').textContent = state.resourceStatus.rules === 'ready' ? `定时 ${state.rules.filter(rule => ruleState(rule, state.targets) === 'timed').length}` : '';
}

function renderMetrics() {
  const metrics = state.metrics || {};
  $('metrics-read-note').hidden = state.resourceStatus.metrics !== 'error';
  $('metrics-read-note').textContent = state.metrics ? '检测统计读取失败，以下为上次读取的数值。请刷新重试。' : '检测统计读取失败，请刷新重试。';
  const percentage = (number) =>
    number == null ? '—' : `${(number <= 1 ? number * 100 : number).toFixed(1)}%`;
  const judged = metrics.judged ?? (metrics.correct != null && metrics.incorrect != null ? metrics.correct + metrics.incorrect : null);
  const prunedUnknown = metrics.unlocated_pruned_samples > 0;
  const stats = [
    [
      '质量通过率',
      percentage(metrics.quality_pass_rate ?? metrics.pass_rate),
      [`正确 ${metrics.correct ?? '—'}`, '/', `可判定 ${judged ?? '—'}`],
    ],
    [
      prunedUnknown ? '评测覆盖率下限' : '评测覆盖率',
      percentage(metrics.coverage ?? metrics.coverage_rate),
      [`已覆盖 ${metrics.covered ?? judged ?? '—'}`, '/', `${prunedUnknown ? '计划至多' : '计划'} ${metrics.planned ?? '—'}`],
      '', prunedUnknown ? `${metrics.unlocated_pruned_samples} 条历史裁剪记录缺少样本位置` : '',
    ],
    ['结论未知', metrics.unknown ?? '—', ['不计为答错', '·', `明确答错 ${metrics.incorrect ?? '—'}`]],
    [
      '已知用量',
      (metrics.known_tokens ?? metrics.total_tokens) == null
        ? '—'
        : Number(metrics.known_tokens ?? metrics.total_tokens).toLocaleString(),
      [`用量未知 ${metrics.unknown_usage ?? '—'}`, '·', `未采集事件 ${metrics.dropped_usage_events ?? metrics.dropped_events ?? '—'}`],
      'token',
    ],
  ];
  $('metrics').innerHTML = stats
    .map(
      ([name, total, [left, divider, right], unit, note]) =>
        `<article class="metric"><span class="metric-label">${escapeHTML(name)}</span>
          <div class="metric-amount"><strong class="metric-value"${total === '—' ? ' data-missing' : ''}>${escapeHTML(total)}</strong>${unit && total !== '—' ? `<span class="metric-unit">${escapeHTML(unit)}</span>` : ''}</div>
          <small><span>${escapeHTML(left)}</span> <span class="metric-divider" aria-hidden="true">${divider}</span> <span>${escapeHTML(right)}</span></small>${note ? `<small>${escapeHTML(note)}</small>` : ''}</article>`,
    )
    .join('');
}

async function allPages(path, key) {
  const list = [], seen = new Set(); let total;
  for (;;) {
    const page = await api(`${path}?limit=100&offset=${list.length}`);
    if (!Array.isArray(page.items) || !Number.isSafeInteger(page.total) || page.total < 0 || (total !== undefined && total !== page.total)) throw Error('目录已变化，请刷新。');
    total = page.total;
    for (const row of page.items) { const id = row[key]; if (!id || seen.has(id)) throw Error('目录已变化，请刷新。'); seen.add(id); list.push(row); }
    if (list.length === total) return { items:list, total };
    if (!page.items.length || list.length > total) throw Error('目录读取不完整，请刷新。');
  }
}

async function loadActivity() {
  const sequence = ++state.reads.activity;
  const query = new URLSearchParams({limit: '20', source: 'scheduled_probe', offset: String(state.batchOffset), view: 'summary'});
  if (state.scopeRule) query.set('rule_id', state.scopeRule);
  if (state.scopeBatch) query.set('scope_batch_id', state.scopeBatch);
  try {
    const page = await api(`/runs?${query}`);
    if (sequence !== state.reads.activity) return;
    const total = page.total ?? 0;
    const lastOffset = Math.max(0, Math.floor((total - 1) / 20) * 20);
    if (total && state.batchOffset > lastOffset) {
      state.batchOffset = lastOffset;
      return loadActivity();
    }
    const offset = total ? page.offset ?? state.batchOffset : 0;
    state.batchOffset = offset;
    state.activityTotal = total;
    state.activityRows = items(page);
    state.resourceStatus.activity = 'ready';
    $('ops-batch-summary').textContent = total ? `${state.scopeBatch ? '本批次 ' : ''}${offset + 1}–${Math.min(offset + 20, total)} / ${total}` : '0 条运行记录';
    $('ops-batch-prev').disabled = offset === 0;
    $('ops-batch-next').disabled = offset + 20 >= total;
    patchView($('operations-history'), renderQualityRuns(state.activityRows, state.rules, credentialName));
  } catch (error) {
    if (sequence !== state.reads.activity || isDiscarded(error)) return;
    state.resourceStatus.activity = 'error';
    throw error;
  } finally {
    if (sequence === state.reads.activity) {
      renderActivityState();
      updateFreshness();
    }
  }
}

async function loadOperations() {
  const sequence = ++state.reads.operations;
  const epoch = client.epoch;
  const current = () => epoch === client.epoch && sequence === state.reads.operations;
  const resources = ['rules', 'actions', 'metrics', 'activity', 'targets', 'settings'];
  for (const resource of resources) if (!state.resourceStatus[resource]) state.resourceStatus[resource] = 'loading';
  if (!state.rules.length) renderRules();
  renderActivityState();
  // Apply each read as it settles so a slow statistic cannot delay a credential restriction.
  async function read(resource, request, render) {
    let data;
    try {
      const result = await request;
      if (!result || typeof result !== 'object' || Array.isArray(result)) throw Error('管理数据格式无效，请刷新重试。');
      data = result;
    }
    finally {
      if (current()) {
        state.resourceStatus[resource] = data === undefined ? 'error' : 'ready';
        render(data);
        updateFreshness();
      }
    }
  }
  const results = await Promise.allSettled([
    read('rules', allPages('/rules', 'id'), data => {
      if (data) state.rules = items(data);
      if (state.activityRows) patchView($('operations-history'), renderQualityRuns(state.activityRows, state.rules, credentialName));
      renderRules();
      renderActivityState();
    }),
    read('actions', allPages('/actions', 'action_id'), data => {
      if (data) state.actions = items(data);
    }),
    read('metrics', api('/metrics?view=summary'), data => {
      if (data) state.metrics = data;
      renderMetrics();
    }),
    loadActivity(),
    read('targets', allPages('/targets', 'auth_id'), data => {
      state.targetsFresh = data !== undefined;
      if (data) state.targets = items(data);
      renderTargetSelectors();
      renderRules();
      if (state.activityRows) patchView($('operations-history'), renderQualityRuns(state.activityRows, state.rules, credentialName));
      if (state.runs.length) renderHistory();
    }),
    read('settings', api('/settings'), data => {
      state.settings = data || {};
      renderRules();
      if (state.editor) updateRuleFields();
    }),
  ]);
  if (!current()) return;
  // Reconciliation callers still require positive reads of rules, actions and credentials.
  const failed = results.slice(0, 5).find(result => result.status === 'rejected');
  if (failed) throw failed.reason;
}

async function refresh() {
  const sequence = ++state.reads.refresh;
  const runRead = state.reads.run;
  if (!state.resourceStatus.questions) state.resourceStatus.questions = 'loading';
  const [questions, operations] = await Promise.allSettled([api('/questions'), loadOperations(), loadHistory()]);
  if (sequence !== state.reads.refresh) return;
  state.resourceStatus.questions = questions.status === 'fulfilled' ? 'ready' : 'error';
  if (questions.status === 'fulfilled') state.questions = items(questions.value);
  renderQuestionSelectors();
  updateFreshness();
  // The two catalogs validate a new connection. Independent history/metric reads can be retried in that session.
  if (!state.connected && !state.targetsFresh) throw operations.reason || Error('凭证目录读取失败，请刷新重试。');
  if (!state.connected && questions.status === 'rejected') throw questions.reason;
  if (state.run && runRead === state.reads.run) await loadRun(state.run.id || state.run.run_id, false);
}

function errorPolicyPauses() {
  return new Set(state.rules.flatMap(rule => (rule.handling || [])
    .filter(record => record.state === 'disabled' && (record.upstream_condition || record.catalog_missing_model))
    .map(record => record.auth_id)));
}

function scopeTargets(scope, recovery) {
  const stopped = errorPolicyPauses();
  const recoverable = new Set(recovery.map(targetID));
  return state.targets.filter(target => !stopped.has(targetID(target)) &&
    (scope === 'all_credentials' || (!target.manual_disabled && target.runtime_status !== 'disabled') || recoverable.has(targetID(target))));
}

function recoveryCandidates(rule) {
  if (rule?.action !== 'disable_credential' || !rule.auto_restore) return [];
  const owned = new Map((rule.handling || []).map(item => [item.auth_id, item]));
  const heldElsewhere = new Set(state.rules.filter(other => other.id !== rule.id).flatMap(other => (other.handling || []).map(item => item.auth_id)));
  return state.targets.filter(t => t.runtime_status === 'disabled' && t.manual_disabled && !t.retained_pause && !t.quality_paused &&
    (owned.has(targetID(t)) ? qualityRecoveryRecord(owned.get(targetID(t))) : rule.target_scope === 'all_credentials' && !t.credential_handling && !heldElsewhere.has(targetID(t)) && ['codex','openai','openai-compatibility'].includes(t.provider)) &&
    (t.reasons || []).every(reason => ['operator_disabled','manual_disabled'].includes(reason)));
}

function qualityRecoveryRecord(record) {
  return record.state === 'disabled' && !record.upstream_condition && !record.catalog_missing_model;
}

const idByteLength = id => new TextEncoder().encode(id).length;
const validShortcut = id => typeof id === 'string' && id.length > 0 && id.length <= 100 && idByteLength(id) <= 256 && !/[\s,，;；]/u.test(id);
const ruleModelLimit = () => state.settings.max_models || 8;
const draftRuleModels = () => [...new Set([...selectedRuleModels, ...splitRuleModels(value('rule-models'))])];
function ruleModelsProblem(models) {
  if (!models.length || models.length > ruleModelLimit()) return `请指定 1–${ruleModelLimit()} 个检测模型。`;
  return models.some(id => idByteLength(id) > 256) ? '单个模型 ID 不能超过 256 字节。' : '';
}
function ruleSampling(models) {
  const samples = number('rule-samples');
  const control = checked('rule-scope-single') && value('rule-question') === 'random-linear' && !!value('rule-control-target');
  const limit = state.settings.max_samples || 32;
  return { limit, over: models.length * samples * (control ? 2 : 1) > limit };
}

function loadRuleShortcuts() {
  if (shortcutsLoaded) return;
  shortcutsLoaded = true;
  try {
    const saved = JSON.parse(localStorage.getItem(shortcutStorage) || '[]');
    if (!Array.isArray(saved)) throw new Error('Invalid shortcut storage');
    customShortcuts = [...new Set(saved.filter(id => validShortcut(id) && !ruleModelShortcuts.includes(id)))];
  } catch { shortcutStorageNote = '无法读取浏览器快捷项，内置模型仍可使用。'; }
}
function saveRuleShortcuts(models) {
  customShortcuts = [...new Set(models.filter(id => !ruleModelShortcuts.includes(id)))];
  try {
    localStorage.setItem(shortcutStorage, JSON.stringify(customShortcuts));
    shortcutStorageNote = '';
  } catch { shortcutStorageNote = '浏览器不允许保存，快捷项仅在本页生效。'; }
  $('rule-shortcut-status').textContent = shortcutStorageNote;
  renderRuleModelControls();
}

const modelGlyph = name => `<svg class="control-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="${{plus:'M12 5v14M5 12h14', check:'m20 6-11 11-5-5', x:'M18 6 6 18M6 6l12 12'}[name]}"/></svg>`;
function modelTokens(models, action, label) {
  return models.map(id => {
    const safe = escapeHTML(id);
    return `<li class="rule-model-token" data-key="id:${safe}"><code>${safe}</code><button type="button" data-${action}="${safe}" aria-label="${label} ${safe}" title="${label} ${safe}">${modelGlyph('x')}</button></li>`;
  }).join('');
}
function renderRuleModelControls() {
  const effective = draftRuleModels(), limit = ruleModelLimit();
  const pending = effective.length - selectedRuleModels.length;
  $('rule-model-count').textContent = `已选 ${selectedRuleModels.length}/${limit}${pending ? ` · 待添加 ${pending}` : ''}`;
  patchView($('rule-model-selection'), modelTokens(selectedRuleModels, 'remove-model', '移除模型'));
  patchView($('rule-model-shortcuts'), [...ruleModelShortcuts, ...customShortcuts].map(id => {
    const safe = escapeHTML(id), selected = effective.includes(id);
    return `<button class="rule-shortcut" data-key="quick:${safe}" type="button" data-add-model="${safe}" aria-pressed="${selected}" ${selected || effective.length >= limit ? 'disabled' : ''}>${modelGlyph(selected ? 'check' : 'plus')}<code>${safe}</code></button>`;
  }).join(''));
  patchView($('rule-custom-shortcuts'), modelTokens(customShortcuts, 'remove-shortcut', '移除快捷项'));
  $('rule-model-add').disabled = !value('rule-models');
  $('rule-shortcut-save').disabled = !value('rule-shortcut-draft');
}

function changeRuleModels(models, consumeDraft = false) {
  selectedRuleModels = models;
  if (consumeDraft) $('rule-models').value = '';
  $('rule-models').dispatchEvent(new Event('input', {bubbles: true}));
}
function addRuleModels(ids, consumeDraft = false) {
  const models = [...new Set([...selectedRuleModels, ...ids])];
  const problem = ruleModelsProblem([...new Set([...models, ...splitRuleModels(value('rule-models'))])]);
  if (problem) { markFieldError('rule-models', problem); return; }
  changeRuleModels(models, consumeDraft);
  $('rule-models').focus({ preventScroll: true });
}

const keepingLegacySchedule = () => !$('rule-legacy-schedule').hidden && !checked('rule-convert-cron');

// Validate before locking the form so the first invalid control can receive focus.
function validateRuleForm(form) {
  const messages = { 'rule-name': '请填写规则名称。', 'rule-target': '请选择目标凭证。',
    'rule-samples': '每模型样本数须为 1–8 的整数。', 'rule-cron': '请填写五段 Cron 检测周期。',
    'rule-max-results': '保留检测结果数须为 1–200 的整数。' };
  for (const field of form.elements) {
    if (!field.willValidate || (field.validity.valid && (field.id !== 'rule-name' || field.value.trim()))) continue;
    markFieldError(field.id, messages[field.id] || field.validationMessage);
    return false;
  }
  return true;
}

function updateRuleSchedule() {
  const legacy = keepingLegacySchedule(), cron = $('rule-cron');
  $('rule-schedule-fields').hidden = legacy;
  cron.disabled = legacy;
  if (legacy) clearFieldError(cron);
  const rule = state.rules.find(r => r.id === value('rule-id'));
  $('rule-schedule-note').textContent = rule && ruleHeld(rule) ? '凭证暂停归属未解除，检测已停止。'
    : checked('rule-timer') ? '保存后从下个周期开始，仅运行本条规则。'
    : rule?.enabled === false ? '原规则已暂停，保存不会恢复检测。' : '关闭时仅手动执行。';
}

let previewRead;
async function updateRulePreview(question) {
  const preview = $('rule-preview'), toggle = $('rule-preview-toggle'), answer = $('rule-preview-answer'), editor = state.editor;
  toggle.hidden = !question || question.kind === 'custom';
  if (toggle.hidden) toggle.setAttribute('aria-expanded', 'false');
  preview.hidden = toggle.getAttribute('aria-expanded') !== 'true';
  const random = question?.kind === 'random';
  $('rule-preview-prompt').textContent = random ? '执行时生成本轮题面，同一范围批次使用同一道题。' : question?.prompt || '未提供题面';
  if (preview.hidden || !editor || question?.kind !== 'logic') {
    previewRead = null;
    answer.textContent = random ? '执行时由本地求解器计算' : '无固定参考答案';
    return;
  }
  if (previewRead?.question === question && previewRead.editor === editor) return;
  const read = previewRead = {question, editor};
  const current = () => previewRead === read && state.editor === editor && !preview.hidden && value('rule-question') === question.id;
  answer.textContent = '读取中…';
  try {
    const record = await api(`/rules/question-preview?id=${encodeURIComponent(question.id)}`);
    if (!current()) return;
    if (record.id !== question.id || record.version !== question.version || record.prompt !== question.prompt || !record.answer) throw Error();
    answer.textContent = record.answer;
  } catch {
    if (current()) answer.textContent = '未取得此版本的参考答案，收起后可重试。';
  }
}

function updateRuleQuestion(all, target) {
  const question = state.questions.find(q => q.id === value('rule-question'));
  const custom = value('rule-question') === 'custom', random = question?.kind === 'random';
  const original = state.rules.find(r => r.id === value('rule-id'));
  const saved = original?.config || original;
  const official = state.settings.capabilities?.mode === 'official_observation';
  const mode = $('rule-judge-mode');
  const llm = mode.querySelector('[value="llm"]'), deterministic = mode.querySelector('[value="deterministic"]');
  const hasOracle = question?.kind === 'logic' || random;
  // An unrelated edit must not rewrite a historical judge or control configuration.
  const sameQuestion = !!saved && saved.question_id === value('rule-question');
  const keepJudge = sameQuestion && saved.judge?.mode === mode.value;
  llm.disabled = all || (official && !(sameQuestion && saved.judge?.mode === 'llm'));
  deterministic.disabled = !!question && !hasOracle && !(keepJudge && mode.value === 'deterministic');
  if ((mode.value === 'llm' && llm.disabled) || (mode.value === 'deterministic' && deterministic.disabled)) mode.value = hasOracle ? 'deterministic' : 'manual';
  const unsupported = official && !all && (mode.value === 'llm' || (random && value('rule-control-target')));
  const note = unsupported ? '原规则依赖独立凭证；官方 CPA 无法核验独立身份，不能执行该配置。'
    : !hasOracle && question ? (mode.value === 'deterministic' ? '此题没有可自动判定的整数答案；原判题设置只会给出「无法判断」。' : '此题没有可自动判定的整数答案。')
    : !all && official && target ? '官方 CPA 无法核验独立凭证身份，不提供独立模型判题。' : '';
  $('rule-judge-note').textContent = note; $('rule-judge-note').hidden = !note;
  void updateRulePreview(question);
  $('rule-prompt-field').hidden = !custom; $('rule-standard-field').hidden = !custom;
  $('rule-control-fields').classList.toggle('hidden', all || !random || (official && !saved?.control_auth_id));
  $('rule-judge-fields').hidden = mode.value !== 'llm';
  for (const id of ['rule-prompt', 'rule-review-standard']) updateQuestionTextValidity($(id), custom);
  if ($('rule-control-fields').classList.contains('hidden')) clearFieldError($('rule-control-target'));
  if ($('rule-judge-fields').hidden) { clearFieldError($('rule-judge-target')); clearFieldError($('rule-judge-model')); }
}

function ruleHandling() { return $('rule-form').elements.namedItem('rule-action').value || 'observe_only'; }

$('rule-upstream-options').innerHTML = Object.entries(upstreamConditions).map(([key, label]) =>
  `<label class="setting-field">${label}<select id="rule-upstream-${key}" aria-describedby="rule-upstream-note">${Object.entries(credentialActions).map(([action, name]) => `<option value="${action}">${name}</option>`).join('')}</select></label>`
).join('');

function upstreamActions() {
  return Object.fromEntries(Object.keys(upstreamConditions).map(key => [key, value(`rule-upstream-${key}`)]));
}

function updateRuleHandling() {
  const action = ruleHandling(), pause = action === 'disable_credential';
  const errorActions = Object.values(upstreamActions());
  const managing = handlesCredential(action) || errorActions.some(handlesCredential);
  const supported = state.settings?.capabilities?.credential_management === true;
  for (const input of $('rule-form').querySelectorAll('[name="rule-action"]')) input.disabled = input.value !== 'observe_only' && !supported;
  for (const option of $('rule-upstream-options').querySelectorAll('option')) option.disabled = handlesCredential(option.value) && !supported;
  $('rule-restore-option').hidden = !pause;
  $('rule-auto-restore').disabled = !pause;
  $('rule-restore-detail').textContent = $('rule-scope-directory').checked
    ? '本规则因答错暂停的凭证和无处置归属的人工停用凭证可复检。整轮全对且停用快照匹配时启用。'
    : '本规则因答错暂停的凭证可复检。整轮全对且停用快照匹配时恢复。';
  $('rule-handling-note').textContent = !handlesCredential(action) ? '答错时仅记录检测结果。'
    : action === 'delete_credential' ? '整轮结果完整且出现答错时，从 CPA 删除对应凭证；不会自动重新上传。网络错误与未知结果不触发删除。'
    : '整轮结果完整且出现答错时暂停对应凭证；网络错误与未知结果不触发暂停。';
  let note = '“指定模型不支持”可依据可信 CPA 目录缺少所选模型，或上游 400/404 明确返回模型不支持。普通限速、超时和未识别错误只记录。';
  if (errorActions.includes('disable_credential')) note += '按错误规则暂停后，凭证退出后续检测。';
  if (!supported) note += '当前宿主未提供凭证处置。';
  else if (managing) {
    const hasControl = value('rule-control-target') && !$('rule-control-fields').classList.contains('hidden');
    note += value('rule-judge-mode') !== 'deterministic' || hasControl ? '管理探测需使用标准答案判定，并清除对照凭证。' : 'OpenAI / Codex 文件凭证通过 CPA 管理接口答题。';
  }
  $('rule-upstream-note').textContent = note;
}

function updateRuleFields() {
  const scope = $('rule-form').elements.namedItem('rule-scope').value;
  const all = !!dynamicScopeLabel(scope);
  $('rule-target-field').hidden = all; $('rule-target').required = !all; $('rule-target').disabled = all;
  if (all) clearFieldError($('rule-target'));
  const selected = draftRuleModels(), target = findTarget(value('rule-target'));
  updateRuleQuestion(all, target);
  const saved = state.rules.find(r => r.id === value('rule-id'));
  const restore = ruleHandling() === 'disable_credential' && checked('rule-auto-restore');
  const recovery = all && restore ? recoveryCandidates({ ...saved, target_scope: scope, action: 'disable_credential', auto_restore: true }) : [];
  const candidates = all ? scopeTargets(scope, recovery) : [target];
  const models = state.targetsFresh ? [...new Set([...candidates.flatMap(availableModels), ...(recovery.length ? saved?.models || [] : [])])].sort() : [];
  let scopeNote = '请选择一个 CPA 凭证。';
  if (!state.targetsFresh) scopeNote = '目录未确认，执行时读取当前范围。';
  else if (all) {
    scopeNote = `当前范围 ${candidates.length} 个凭证。目录未确认的凭证也纳入检测。`;
    if (scope === 'all_credentials') scopeNote += restore ? `待复检的停用凭证 ${recovery.length} 个（含无归属的人工停用）。` : '人工禁用的凭证按权限跳过。';
    else if (recovery.length) scopeNote += `含本规则因答错暂停的 ${recovery.length} 个凭证。`;
  } else if (target) {
    scopeNote = errorPolicyPauses().has(targetID(target)) ? '已按错误规则暂停，退出后续检测。'
      : !models.length ? '模型目录未确认，执行时检测。'
      : selected.some(id => !models.includes(id)) ? '当前目录未注册部分已选模型，执行时核实并按模型不支持策略处理。' : '仅使用指定凭证。';
  }
  $('rule-scope-note').textContent = scopeNote;
  const picker = $('rule-model-catalog');
  const options = `<option value="">${models.length ? '从凭证目录添加' : '凭证目录暂无模型'}</option>` + models.map(m => `<option value="${escapeHTML(m)}" ${selected.includes(m) ? 'disabled' : ''}>${escapeHTML(m)}</option>`).join('');
  patchView(picker, options);
  picker.disabled = !models.length || selected.length >= ruleModelLimit();
  picker.title = !models.length ? '凭证目录暂无模型' : picker.disabled ? '已达检测模型上限' : '从凭证目录添加模型';
  renderRuleModelControls();
  const samples = number('rule-samples');
  const validSamples = Number.isInteger(samples) && samples >= 1 && samples <= 8;
  for (const button of $('rule-form').querySelectorAll('[data-sample-step]')) {
    button.disabled = validSamples && (button.dataset.sampleStep === '1' ? samples === 8 : samples === 1);
  }
  const byCount = value('rule-retention-mode') === 'count';
  const countField = $('rule-max-results');
  countField.hidden = countField.disabled = !byCount;
  countField.required = byCount;
  const limit = number('rule-max-results'), round = selected.length * samples, retention = $('rule-retention-note');
  retention.textContent = byCount && validSamples && limit > 0 && limit < round ? `为保留一整轮结果，保存时至少保留 ${round} 条。` : '';
  retention.hidden = !retention.textContent;
  updateRuleSchedule();
  updateRuleHandling();
}

function editRule(rule = {}) {
  const config = rule.config || rule;
  loadRuleShortcuts();
  selectedRuleModels = rule.id ? [...(config.models || [])] : [ruleModelShortcuts[0]];
  $('rule-models').value = $('rule-shortcut-draft').value = '';
  $('rule-shortcut-panel').hidden = true;
  $('rule-shortcut-toggle').setAttribute('aria-expanded', 'false');
  $('rule-shortcut-status').textContent = shortcutStorageNote;
  $('rule-form').querySelectorAll('[aria-invalid="true"]').forEach(clearFieldError);
  $('rule-form').querySelector('.dialog-message')?.remove();
  $('rule-preview-toggle').setAttribute('aria-expanded', 'false');
  $('rule-dialog-title').textContent = rule.id ? '编辑检测规则' : '新建检测规则';
  const legacy = !!rule.id && (!!rule.timezone || rule.interval_seconds > 0);
  $('rule-legacy-schedule').hidden = !legacy;
  $('rule-original-plan').textContent = !legacy ? '' : rule.cron ? `${rule.cron} · ${rule.timezone || '服务端时区'}` : `每 ${rule.interval_seconds} 秒`;
  $('rule-convert-cron').checked = false;
  const effort = rule.id ? config.parameters?.reasoning_effort || '' : 'high';
  const efforts = $('rule-form').elements.namedItem('rule-effort');
  const legacyEffort = !Array.from(efforts).some(radio => radio.value && radio.value === effort);
  $('rule-legacy-effort').hidden = !legacyEffort;
  $('rule-legacy-effort').querySelector('input').disabled = !legacyEffort;
  $('rule-legacy-effort').querySelector('span').textContent = effort || '未指定';
  for (const radio of efforts) radio.checked = radio.value === (legacyEffort ? '' : effort);
  const legacyRetention = !!rule.id && !rule.max_results && rule.retention_days > 0;
  $('rule-retention-mode').querySelector('[value="legacy"]').hidden = !legacyRetention;
  const values = {
    id: rule.id || '',
    revision: rule.revision || '',
    name: rule.name || '',
    target: config.auth_id || '',
    question: config.question_id || 'logic-candy',
    samples: config.samples || 1,
    prompt: config.prompt || '',
    'review-standard': config.review_standard || '',
    'judge-mode': config.judge?.mode || 'deterministic',
    'judge-target': config.judge?.auth_id || '',
    'control-target': config.control_auth_id || '',
    'judge-model': config.judge?.model || '',
    cron: rule.cron || $('rule-cron').defaultValue,
    'retention-mode': legacyRetention ? 'legacy' : rule.retention_mode || 'count',
    'max-results': rule.max_results || $('rule-max-results').defaultValue,
  };
  for (const [name, current] of Object.entries(values)) $(`rule-${name}`).value = current;
  $('rule-form').elements.namedItem('rule-scope').value = rule.id ? rule.target_scope || 'single' : 'all_enabled';
  const held = ruleHeld(rule);
  $('rule-form').elements.namedItem('rule-action').value = ['disable_credential','delete_credential'].includes(rule.action) ? rule.action : 'observe_only';
  for (const key of Object.keys(upstreamConditions)) $(`rule-upstream-${key}`).value = rule.upstream_error_actions?.[key] || 'observe_only';
  $('rule-auto-restore').checked = rule.auto_restore === true;
  const timer = $('rule-timer');
  timer.checked = !held && rule.timer_active === true;
  timer.dataset.initial = String(timer.checked);
  timer.disabled = held || (!!rule.id && typeof rule.timer_active !== 'boolean');
  updateRuleFields();
  $('rule-hold-note').classList.toggle('hidden', !held);
  setPage('operations');
  openEditor('rule-dialog');
}

async function saveRule() {
  const config = runConfig('rule-');
  const body = {
    ...config,
    name: value('rule-name'),
    action: ruleHandling(),
    upstream_error_actions: upstreamActions(),
    auto_restore: ruleHandling() === 'disable_credential' && checked('rule-auto-restore'),
    activation: String(checked('rule-timer')) === $('rule-timer').dataset.initial ? 'keep' : checked('rule-timer') ? 'start' : 'stop',
    automatic_pause: false,
    fail_threshold: state.rules.find(r => r.id === value('rule-id'))?.fail_threshold || 3,
  };
  if (handlesCredential(body.action) || Object.values(body.upstream_error_actions).some(handlesCredential)) {
    if (!state.settings?.capabilities?.credential_management) throw Error('当前宿主未提供凭证处置，请刷新后重试。');
    if (config.judge.mode !== 'deterministic') fieldError('rule-judge-mode', '暂停或移除凭证需使用标准答案判定。');
    if (config.control_auth_id) fieldError('rule-control-target', '管理探测需清除对照凭证。');
  }
  const retentionMode = value('rule-retention-mode');
  if (retentionMode !== 'legacy') body.retention_mode = retentionMode;
  if (retentionMode === 'count') {
    const requested = number('rule-max-results');
    if (!Number.isInteger(requested) || requested < 1 || requested > 200) fieldError('rule-max-results', '保留检测结果数须为 1–200 的整数。');
    body.max_results = Math.max(requested, config.models.length * config.samples);
  }
  if (!keepingLegacySchedule()) {
    const cron = value('rule-cron').split(/\s+/);
    if (cron.length !== 5 || cron.some(field => /[@=]/.test(field))) fieldError('rule-cron', '请输入五段 Cron：分、时、日、月、星期，不含秒或时区。');
    body.cron = cron.join(' ');
  }
  if (value('rule-id')) Object.assign(body, { id: value('rule-id'), revision: number('rule-revision') });
  try { await api('/rules', body.id ? 'PUT' : 'POST', body); }
  catch (error) {
    if (error.code === 'invalid_cron' && !keepingLegacySchedule()) fieldError('rule-cron', '检测周期无效，请核对各段的取值、步长和日期。');
    throw error;
  }
  closeSurface('rule-dialog', true);
  if (state.page === 'operations') notice(`规则“${body.name}”已保存。${body.activation === 'start' ? '将从下个周期开始检测。' : ''}`);
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

function exportRun(run) {
  if (!run) return;
  const blob = new Blob([JSON.stringify(run, null, 2)], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `model-quality-${run.id || run.run_id || 'run'}.json`;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 0);
}

async function dispatchClick(event) {
  const button = event.target.closest('button,[data-run]');
  if (!button) return;
  if (button.dataset.deleteRun) { confirmHistoryDelete(button.dataset.deleteRun); return; }
  if (button.dataset.close) {
    closeSurface(button.dataset.close);
    if (button.dataset.close === 'release-dialog') { state.reads.preview++; state.releaseBlocked = true; }
    if (button.dataset.close === 'preview-dialog') $('safe-preview').srcdoc = '';
    return;
  }
  const manualView = button.dataset.manualView || button.dataset.manualPane;
  if (manualView) {
    setManualView(manualView, !button.dataset.manualPane);
    if (manualView === 'history' && state.connected) await loadHistory();
    return;
  }
  if (button.id === 'reading-toggle') {
    setReadingExpanded(!state.readingExpanded);
    button.focus({ preventScroll: true });
    return;
  }
  if (button.hasAttribute('data-rule-new')) { protectEditor(() => editRule()); return; }
  if (button.hasAttribute('data-rule-clear')) { $('rule-search').value = ''; $('rule-filter').value = 'all'; renderRules(); $('rule-search').focus(); return; }
  if (button.dataset.ruleTimer) {
    const rule = state.rules.find(item => item.id === button.dataset.ruleTimer);
    if (!rule || button.disabled) return;
    await safely(async () => {
      renderRules();
      try {
        const updated = await api('/rules/timer', 'PUT', { id: rule.id, revision: rule.revision, enabled: !rule.timer_active });
        Object.assign(rule, updated);
      } catch (error) {
        if (isDiscarded(error)) throw error;
        await loadOperations().catch(() => {});
        if (error.status === 409) throw new Error('规则状态已变更，请核对列表后重试。');
        throw error;
      }
      await loadOperations();
    }, `rule-timer:${rule.id}`);
    renderRules();
    return;
  }
  if (button.dataset.ruleSelect) {
    state.scopeRule = state.scopeRule === button.dataset.ruleSelect ? '' : button.dataset.ruleSelect;
    selectBatch('', 0, false); await loadActivity(); return;
  }
  if (button.dataset.page) {
    setPage(button.dataset.page);
    return;
  }
  if (button.dataset.run) {
    if (button.closest('#page-operations')) return openQualityDetail(button.dataset.run);
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
  const detail = button.closest('#quality-detail') ? qualityDetail : null;
  const sourceRun = detail ? detail.run : state.run;
  const retainedSample = index => sourceRun.samples[Number(index) - (sourceRun.pruned_samples || 0)];
  if (button.dataset.copySample != null) {
    await navigator.clipboard.writeText(retainedSample(button.dataset.copySample).output);
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
      retainedSample(button.dataset.previewSample).output || '',
    );
    $('preview-dialog').showModal();
    $('safe-preview').srcdoc = preview;
    return;
  }
  if (button.dataset.evidenceSample != null) {
    const sample = retainedSample(button.dataset.evidenceSample);
    const evidenceID = sample.evidence?.auth_id;
    const label = evidenceID && evidenceID === sourceRun.config?.auth_id ? sourceRun.credential_email || credentialName(evidenceID) : credentialName(evidenceID);
    $('evidence-content').innerHTML = renderSampleEvidence(sample, sourceRun.source === 'manual_test', label);
    $('evidence-dialog').showModal();
    return;
  }
  if (button.dataset.reviewSample != null) {
    const id = sourceRun.id || sourceRun.run_id;
    const selection = state.reads.run;
    const current = () => detail ? detail === qualityDetail && detailDialog.open
      : selection === state.reads.run && (state.run?.id || state.run?.run_id) === id;
    const fields = button.closest('.sample-review');
    try {
      await api('/runs/review', 'POST', {
        id, sample_index: Number(button.dataset.reviewSample),
        verdict: fields.querySelector('select').value,
        note: fields.querySelector('input').value.trim(),
      });
    } catch (error) {
      if (current()) throw error;
      return;
    }
    if (!current()) return;
    if (detail) await readQualityDetail(detail);
    else await loadRun(id, false);
    if (button.isConnected) localFeedback(button, '评审已保存');
    return;
  }
  if (button.dataset.ruleEdit) {
    protectEditor(() => editRule(state.rules.find((rule) => rule.id === button.dataset.ruleEdit)));
    return;
  }
  if (button.dataset.ruleRun) {
    const rule = state.rules.find((entry) => entry.id === button.dataset.ruleRun);
    if (!rule) throw new Error('规则当前不可用');
    const config = rule.config || rule, stopped = errorPolicyPauses();
    if ([config.auth_id, config.control_auth_id, config.judge?.auth_id].some(id => stopped.has(id))) throw new Error('已按错误规则暂停，退出后续检测。');
    const recovery = rule?.action === 'disable_credential' && rule.auto_restore && rule.handling?.some(h => h.auth_id === rule.auth_id && qualityRecoveryRecord(h));
    const target = findTarget(config.auth_id);
    const checkTarget = target?.retained_pause || target?.quality_paused || (target?.manual_disabled && !recovery);
    const blocked = blockedConfig({ ...config, auth_id: checkTarget ? config.auth_id : '' });
    if (blocked) throw new Error(blocked);
    if (!rule?.enabled || ruleHeld(rule))
      throw new Error('检测已停止。请先人工解除凭证暂停，再显式恢复检测规则。');
    const run = () => api('/rules/run', 'POST', { id: rule.id, revision: rule.revision });
    if (dynamicScopeLabel(rule.target_scope)) {
      const recoverable = recoveryCandidates(rule), candidates = scopeTargets(rule.target_scope, recoverable);
      const directory = `当前范围 ${candidates.length} 个凭证。` + (rule.target_scope === 'all_credentials' ? (rule.action === 'disable_credential' && rule.auto_restore ? '无处置归属的人工停用凭证整轮全对且快照匹配时可启用。' : '人工禁用的凭证按权限跳过。') : '');
      const upstream = configuredUpstreamActions(rule.upstream_error_actions);
      const managed = handlesCredential(rule.action) || upstream.length > 0;
      const configured = [
        ...(handlesCredential(rule.action) ? [`整轮可判且有答错时${credentialActions[rule.action]}`] : []),
        ...upstream.map(([condition, action]) => `${condition}时${action}`),
      ];
      const samples = candidates.length * (rule.models || []).length * rule.samples;
      const summary = managed ? `本规则待复测 ${recoverable.length} 个。已保存的处理策略为${configured.join('，')}。` : `最多计划 ${samples} 份目标回答，实际答题将消耗调用额度。`;
      actionDialog({ title: '执行范围检测？', description: directory + summary + '执行时核实凭证和模型。目录确认缺少所选模型时按模型不支持策略处理。', confirm: '开始检测', submit: async () => {
        let receipt;
        try {
          receipt = await run();
          if (!Number.isSafeInteger(receipt?.count) || receipt.count < 0 || !Number.isSafeInteger(receipt.skipped) || receipt.skipped < 0 || (receipt.count > 0 && !receipt.scope_batch_id)) throw Error('invalid receipt');
        } catch (error) {
          if (isDiscarded(error) || (error.status >= 400 && error.status < 500 && !error.outcomeUnknown)) throw error;
          // Do not leave a retry button when the server may already have queued a batch.
          state.scopeRule = rule.id;
          selectBatch('');
          await loadOperations().catch(() => {});
          notice('提交结果未确认。请先核对规则运行记录，不要重复执行。', true);
          return;
        }
        if (receipt.scope_batch_id) selectBatch(receipt.scope_batch_id);
        showScopeReceipt(rule, receipt);
        try { await loadOperations(); }
        catch (error) { if (isDiscarded(error)) throw error; notice('已收到手动执行回执，部分列表读取失败，请刷新。', true, 'activity-read-note'); }
      } }); return;
    }
    const reply = await run();
    await Promise.all([openQualityDetail(reply.run_id || reply.id), loadActivity()]);
    return;
  }
  if (button.dataset.ruleBatch) {
    selectBatch(button.dataset.ruleBatch); await loadActivity(); return;
  }
  if (button.dataset.ruleDelete) {
    const rule = state.rules.find((entry) => entry.id === button.dataset.ruleDelete);
    if (!rule) return;
    actionDialog({ title: '删除检测规则', description: `将删除“${rule.name || rule.id}”及其运行记录和日志。保留 CPA 凭证。此操作无法撤销。`, confirm: '删除规则', danger: true, submit: async () => {
      const epoch = client.epoch;
      const receipt = await cleanupRequest('/rules', { id: rule.id, revision: rule.revision });
      const selected = state.scopeRule === rule.id || (state.scopeBatch && (state.scopeBatch === rule.last_scope_batch || state.activityRows?.some(row => (row.rule_id || row.config?.rule_id) === rule.id)));
      if (receipt) state.rules = state.rules.filter(entry => entry.id !== rule.id);
      if (receipt && selected) {
        state.scopeRule = '';
        selectBatch('', 0, false);
      }
      if (!await refreshCleanupLists(epoch)) return;
      const listed = state.resourceStatus.rules === 'ready' ? state.rules.some(entry => entry.id === rule.id) : null;
      if (!receipt && listed === false && selected) {
        state.scopeRule = '';
        selectBatch('', 0, false);
        try { await loadActivity(); } catch (error) { if (isDiscarded(error)) throw error; }
      }
      if (epoch !== client.epoch || state.page !== 'operations') return;
      if (receipt || listed === false) focusAfterCleanup('ops-all-rules');
      const current = state.resourceStatus.rules === 'ready' && state.resourceStatus.activity === 'ready';
      const message = receipt ? `规则“${rule.name || rule.id}”已删除。` : `删除规则的回执未确认。${listed === null ? '' : listed ? '当前规则仍在列表中。' : '当前列表中已无此规则。'}`;
      notice(message + (current ? '' : '列表读取失败，请刷新核对。'), !receipt || !current, 'ops-all-rules');
    } });
    return;
  }
  if (button.dataset.handlingAck) {
    const rule = state.rules.find(r => r.handling?.some(h => h.action_id === button.dataset.handlingAck));
    const record = rule?.handling.find(h => h.action_id === button.dataset.handlingAck);
    if (!record) throw new Error('处置记录已变化，请刷新。');
    actionDialog({title: '核对人工处理', description: '请先在 CPA 中手动启用或移除该凭证。确认后会重新核对状态、结束本次处置关联，并停止此规则。此操作不会修改 CPA 凭证。', confirm: '核对并停止规则', submit: async () => {
      await api('/rules/handling/acknowledge', 'POST', {rule_id: rule.id, auth_id: record.auth_id, action_id: record.action_id});
      await loadOperations(); notice('人工处理已核对，规则已停止。');
    }});
    return;
  }
  if (button.dataset.actionPreview != null) await previewAction(Number(button.dataset.actionPreview));
}

document.addEventListener('click', (event) => {
  const button = event.target.closest(
    '[data-rule-run],[data-rule-batch],[data-rule-delete],[data-handling-ack],[data-action-preview],[data-review-sample]',
  );
  if (button?.disabled) return;
  safely(() => dispatchClick(event), button ? `action:${JSON.stringify(button.dataset)}` : '', button);
});
document.addEventListener('keydown', (event) => {
  if (['Enter', ' '].includes(event.key) && event.target.matches('[data-run]:not(button)')) { event.preventDefault(); event.target.click(); }
});
$('connection-top').addEventListener('click', () => $('connection-dialog').showModal());
$('connection-open').addEventListener('click', () => $('connection-dialog').showModal());
$('connection-form').addEventListener('submit', (event) => {
  event.preventDefault();
  safely(() => connectManagement(value('management-key')), 'connect', event.submitter);
});
$('disconnect').addEventListener('click', () => disconnect());
$('refresh').addEventListener('click', () => safely(state.page === 'login' ? loginWorkbench.refresh : refresh, 'refresh', $('refresh')));
$('target').addEventListener('change', () => renderTarget(true));
$('models').addEventListener('change', () => { renderReasoning(true); renderTarget(); });
$('reasoning').addEventListener('change', () => { renderReasoning(true); renderAdmission(); });
$('samples').addEventListener('change', renderAdmission);
$('question').addEventListener('change', renderQuestion);
for (const input of $('rule-form').querySelectorAll('[name="rule-action"]')) input.addEventListener('change', updateRuleFields);
$('rule-upstream-options').addEventListener('change', updateRuleFields);
for (const id of ['rule-target','rule-question','rule-scope-all','rule-scope-directory','rule-scope-single','rule-auto-restore','rule-judge-mode','rule-models','rule-samples','rule-control-target','rule-max-results','rule-retention-mode']) $(id).addEventListener('input', updateRuleFields);
for (const id of ['rule-timer','rule-convert-cron','rule-cron']) $(id).addEventListener('input', updateRuleSchedule);
for (const button of $('rule-form').querySelectorAll('[data-sample-step]')) {
  button.addEventListener('click', () => {
    const field = $('rule-samples'), value = field.valueAsNumber;
    field.value = Number.isFinite(value) ? Math.min(8, Math.max(1, Math.trunc(value) + Number(button.dataset.sampleStep))) : 1;
    field.dispatchEvent(new Event('input', { bubbles: true }));
    if (button.disabled) field.focus({ preventScroll: true });
  });
}
$('rule-preview-toggle').addEventListener('click', ({currentTarget: toggle}) => {
  toggle.setAttribute('aria-expanded', String(toggle.getAttribute('aria-expanded') !== 'true'));
  void updateRulePreview(state.questions.find(q => q.id === value('rule-question')));
});
$('rule-model-catalog').addEventListener('change', () => {
  const model = value('rule-model-catalog');
  if (!model) return;
  $('rule-model-catalog').value = '';
  addRuleModels([model]);
});
function commitModelInput() {
  const ids = splitRuleModels(value('rule-models'));
  if (ids.length) addRuleModels(ids, true);
}
$('rule-model-add').addEventListener('click', commitModelInput);
$('rule-models').addEventListener('keydown', event => {
  if (event.key === 'Enter' && !event.shiftKey && !event.isComposing && event.keyCode !== 229) { event.preventDefault(); commitModelInput(); }
});
$('rule-model-shortcuts').addEventListener('click', event => {
  const button = event.target.closest('[data-add-model]');
  if (button && !button.disabled) addRuleModels([button.dataset.addModel]);
});
$('rule-model-selection').addEventListener('click', event => {
  const button = event.target.closest('[data-remove-model]');
  if (!button) return;
  const index = selectedRuleModels.indexOf(button.dataset.removeModel);
  changeRuleModels(selectedRuleModels.filter(id => id !== button.dataset.removeModel));
  const buttons = $('rule-model-selection').querySelectorAll('button');
  (buttons[index] || buttons[index - 1] || $('rule-models')).focus({preventScroll:true});
});
$('rule-shortcut-toggle').addEventListener('click', () => {
  const panel = $('rule-shortcut-panel');
  panel.hidden = !panel.hidden;
  $('rule-shortcut-toggle').setAttribute('aria-expanded', String(!panel.hidden));
  if (!panel.hidden) $('rule-shortcut-draft').focus();
});
$('rule-shortcut-draft').addEventListener('input', () => { $('rule-shortcut-save').disabled = !value('rule-shortcut-draft'); });
$('rule-shortcut-save').addEventListener('click', () => {
  const ids = splitRuleModels(value('rule-shortcut-draft'));
  if (!ids.length || !ids.every(validShortcut)) {
    $('rule-shortcut-status').textContent = '请输入有效模型 ID，每项最多 100 个字符、256 字节。';
    $('rule-shortcut-draft').focus(); return;
  }
  $('rule-shortcut-draft').value = '';
  saveRuleShortcuts([...customShortcuts, ...ids]);
  if (!shortcutStorageNote) $('rule-shortcut-status').textContent = '快捷项已保存。';
  $('rule-shortcut-draft').focus();
});
$('rule-custom-shortcuts').addEventListener('click', event => {
  const button = event.target.closest('[data-remove-shortcut]');
  if (!button) return;
  saveRuleShortcuts(customShortcuts.filter(id => id !== button.dataset.removeShortcut));
  $('rule-shortcut-draft').focus();
});
$('run-form').addEventListener('submit', (event) => {
  event.preventDefault();
  safely(() => submitRun(runConfig()), 'submit-run');
});
$('run-cancel').addEventListener('click', () =>
  safely(async () => {
    const id = state.run.id || state.run.run_id;
    const scope = feedbackScope();
    await api('/runs/cancel', 'POST', { id });
    if (id === (state.run?.id || state.run?.run_id)) {
      if (scope === feedbackScope()) notice('已请求停止，后台正在核对执行进度。');
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
  ),
);
$('run-export').addEventListener('click', () => exportRun(state.run));
$('history-prev').addEventListener('click', () =>
  safely(() => loadHistory(Math.max(0, state.historyOffset - 10))),
);
$('history-next').addEventListener('click', () =>
  safely(() => loadHistory(state.historyOffset + 10)),
);
$('rule-new').addEventListener('click', () => protectEditor(() => editRule()));
$('rule-form').addEventListener('submit', (event) => {
  event.preventDefault();
  if (!validateRuleForm(event.currentTarget)) return;
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
function clearScopeReceipt() {
  state.receiptBatch = '';
  $('ops-dispatch-receipt').hidden = true;
  $('ops-dispatch-receipt').replaceChildren();
}

function showScopeReceipt(rule, receipt) {
  clearScopeReceipt();
  const panel = $('ops-dispatch-receipt');
  const title = `手动执行回执 · ${rule.name || rule.id}`;
  const dispatch = { matched: receipt.count, skipped: receipt.skipped, details: receipt.skip_details, skipReasons: receipt.skip_reasons };
  state.receiptBatch = receipt.scope_batch_id || '';
  patchView(panel, renderScopeDispatch(dispatch, title, rule.id, true));
  const details = panel.querySelector('details');
  if (details) details.open = true;
  panel.hidden = false;
  $('local-feedback').textContent = `${title}。纳入检测 ${receipt.count} 个，跳过 ${receipt.skipped} 个。`;
}

function selectBatch(batch, offset = 0, focus = true) {
  if (!batch || batch !== state.receiptBatch) clearScopeReceipt();
  state.scopeBatch = batch; state.batchOffset = offset;
  state.activityRows = null;
  if (batch) state.scopeRule = '';
  $('ops-batch-bar').hidden = !batch;
  $('ops-batch-summary').textContent = '正在读取记录…';
  $('ops-batch-prev').disabled = $('ops-batch-next').disabled = true;
  state.resourceStatus.activity = 'loading'; patchView($('operations-history'), ''); renderActivityState(); renderRules();
  if (focus && !$('action-dialog').open) {
    $('ops-activity').focus({ preventScroll: true });
    $('ops-activity').scrollIntoView({ block: 'nearest', behavior: 'instant' });
  }
}
for (const [id, shift] of [['ops-batch-prev',-20],['ops-batch-next',20],['ops-batch-clear',0]]) $(id).addEventListener('click', () => {
  selectBatch(shift ? state.scopeBatch : '', shift ? Math.max(0, state.batchOffset + shift) : 0, false);
  safely(loadActivity, 'batch-page');
});
$('ops-history-clear').addEventListener('click', () => actionDialog({
  title: '清空所有规则的运行记录？',
  description: (state.scopeBatch ? '当前显示单个批次。' : state.scopeRule ? `当前显示规则“${state.rules.find(rule => rule.id === state.scopeRule)?.name || state.scopeRule}”。` : '') + '将删除所有规则已结束的检测记录及对应日志。保留进行中和处置所需的记录、手动测试记录及 CPA 凭证。',
  confirm: '清空记录', danger: true,
  submit: async () => {
    const epoch = client.epoch;
    const result = await cleanupRequest('/rules/run-history');
    state.scopeRule = '';
    selectBatch('', 0, false);
    if (!await refreshCleanupLists(epoch) || state.page !== 'operations') return;
    focusAfterCleanup('ops-activity');
    const current = state.resourceStatus.activity === 'ready';
    const message = result ? `已清空 ${result.deleted} 条运行记录。${result.kept ? `保留 ${result.kept} 条进行中或处置所需的记录。` : ''}` : '清空回执未确认，删除条数未知。';
    notice(message + (current ? (result ? '' : '已重新读取全部规则记录。') : '记录读取失败，请刷新核对。'), !result || !current, 'activity-read-note');
  },
}));
for (const id of ['ops-scope-clear', 'ops-all-rules']) $(id).addEventListener('click', () => { state.scopeRule = ''; selectBatch('', 0, false); safely(loadActivity, 'activity-scope'); });
$('rule-search').addEventListener('input', renderRules);
$('rules-list').addEventListener('toggle', event => {
  const node = event.target;
  if (!node.matches('details[data-skip-rule]')) return;
  const dispatch = state.rules.find(rule => rule.id === node.dataset.skipRule)?.last_scope_dispatch;
  if (dispatch) patchView(node.querySelector('.scope-skip-body'), node.open ? renderScopeSkipDetails(dispatch) : '');
}, true);
$('rule-filter').addEventListener('change', renderRules);
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
$('connection-build').textContent = `UI ${JSON.parse($('build-info').textContent).ui_version}`;
document.querySelector('.skip-link').addEventListener('click', event => {
  event.preventDefault();
  $('main').focus();
  $('main').scrollIntoView({ block: 'start' });
});
setPage(location.hash.slice(1));
setManualView('setup');
setReadingExpanded(false);
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
  if (!request || request.cleanupSent || state.busy.has('dialog-action')) return;
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
      if (request.cleanupSent) {
        $('action-dialog').close();
        notice('清理请求已发送，读取结果失败。请刷新核对。', true);
        return;
      }
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
$('action-dialog').addEventListener('close', () => { if (!$('action-dialog').open) { actionRequest = null; $('action-name').value = ''; } });
function clearFieldError(field) {
  if (field.id && field.hasAttribute('aria-invalid')) {
    field.removeAttribute('aria-invalid');
    const errorID = `${field.id}-error`;
    $(errorID)?.remove();
    field.setAttribute('aria-describedby', (field.getAttribute('aria-describedby') || '').split(' ').filter((id) => id !== errorID).join(' '));
  }
}
document.addEventListener('input', (event) => {
  const field = event.target;
  if (field.id === 'prompt') updateQuestionNote();
  else if (field.id === 'rule-prompt' || field.id === 'rule-review-standard') updateQuestionTextValidity(field, value('rule-question') === 'custom');
  else clearFieldError(field);
  if (event.target.closest('#rule-dialog')) updateEditorDirty();
});
document.addEventListener('change', (event) => { if (event.target.closest('#rule-dialog')) updateEditorDirty(); });
for (const id of ['compare-model-a', 'compare-model-b', 'sample-filter']) $(id).addEventListener('change', () => {
  if (value('compare-model-a') === value('compare-model-b')) $('compare-model-b').value = '';
  applyComparison();
});
$('preview-dialog').addEventListener('close', () => { $('safe-preview').srcdoc = ''; });
$('release-dialog').addEventListener('close', () => { state.reads.preview++; state.releaseBlocked = true; });
window.addEventListener('hashchange', () => setPage(location.hash.slice(1)));
window.addEventListener('resize', scheduleWorkspaceLayout);
const workspaceLayoutObserver = new ResizeObserver(scheduleWorkspaceLayout);
for (const selector of ['.app-header', '.primary-nav', '.quality-heading', '.quality-overview', '.manual-pane-switch', '#notice']) workspaceLayoutObserver.observe(document.querySelector(selector));
renderMetrics();
renderRules();
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
