import { escapeHTML, date } from './view.js';
import { isDiscarded } from './client.js';

const labels = {
  queued: '排队中', running: '登录中', waiting_browser: '等待浏览器授权', exchanging: '兑换授权',
  validating: '核验身份', awaiting_commit: '等待确认导入', committing: '保存中', committed: '已保存',
  saved_not_adopted: '已保存，尚未采用', commit_unknown: '提交结果待核对', interaction_required: '登录未完成',
  failed: '登录失败', conflict: '身份或状态冲突', cancelled: '已取消', cancelling: '正在取消',
  discarded: '已丢弃候选', expired: '已过期', interrupted: '已中断', succeeded: '操作完成', security_uncertain: '安全操作待核对',
};
const activeStates = new Set(['queued', 'running', 'waiting_browser', 'exchanging', 'validating', 'committing', 'cancelling']);
const cancellableStates = new Set(['queued', 'running', 'waiting_browser', 'exchanging', 'validating']);
const reasons = {
  invalid_request: '请求格式不正确，请核对输入后重新操作。',
  invalid_proxy: '代理地址无效。请填写 HTTP、HTTPS、SOCKS5、SOCKS5H 地址，或 direct 直连；不要包含路径、查询参数或片段。',
  proxy_auth_unsupported: 'SOCKS 代理暂不支持用户名和密码，请使用无认证的 SOCKS 代理或 HTTP / HTTPS 代理。',
  invalid_credentials: '请提供完整邮箱、密码和 TOTP 长期密钥。',
  credentials_rejected: '密码被拒绝，请核对后手动创建新的登录任务。',
  unsupported_login_target: '此凭证来源不支持重新授权。',
  unsupported_login_store: '当前宿主的凭证存储不支持可靠导入。',
  quality_authority_unavailable: '宿主的持久状态尚未就绪，暂时无法导入凭证。',
  home_controlled: '当前 Home 管理模式不支持此操作。',
  runtime_invalid: '登录运行环境校验失败，请检查安装。',
  runtime_failed: '本机登录运行环境未完成请求，请检查安装后手动重试。',
  runtime_timeout: '登录等待超时，未自动重试。',
  runtime_protocol_error: '本机登录运行环境返回了无效结果，未采用授权。',
  rate_limited: '此账号已达到手动登录频率限制，请稍后再试。',
  identity_conflict: '本次登录身份与目标凭证不一致，未更新凭证。',
  login_commit_pending: '此凭证还有待核对的提交，请先由宿主完成对账。',
  invalid_callback: '回调地址不符合本次授权会话，请复制完整回调地址。',
  already_consumed: '此授权回调已经处理，请刷新任务核对结果。',
  exchange_failed: '官方授权兑换未完成，未自动重试。',
  storage_unavailable: '登录状态存储暂不可用，请核对宿主状态。',
  storage_capacity: '登录记录已达到容量上限，请由宿主管理员整理归档后重试。',
  vault_conflict: '保存资料正在使用或版本已变化，请等待任务完成后刷新。',
  vault_corrupt: '登录资料无法读取，请由宿主管理员核对存储与备份。',
  vault_identity_changed: '保存资料对应的凭证身份已变化，请更新资料后再使用。',
  idempotency_conflict: '请求标识已用于不同操作，请刷新任务核对。',
  idempotency_unknown: '上次请求结果尚未确认，请刷新任务核对，避免重复登录。',
  interrupted: '宿主重启后任务已中断，未自动重试。',
  runtime_unavailable: '本机登录运行环境尚未就绪，请检查安装后重试。',
  runtime_hash_mismatch: '登录运行环境校验失败，请检查安装。',
  runtime_unsupported: '当前平台不支持本地登录运行环境。',
  unsupported_method: '当前宿主不支持此登录方式。',
  unsupported_credential_source: '此凭证来源不支持重新授权。',
  unsupported_source: '此凭证来源不支持重新授权。',
  unsupported_provider: '此凭证不是受支持的 Codex OAuth 凭证。',
  credential_source: '此凭证来源不支持重新授权。',
  home_unsupported: '当前 Home 管理模式不支持此操作。',
  plugin_unavailable: '当前插件实例不可用，请重新连接后核对。',
  capability_unavailable: '插件登录服务暂不可用，请检查插件配置与运行包。',
  password_rejected: '密码被拒绝，请核对后手动创建新的登录任务。',
  totp_rejected: '二次验证被拒绝，请核对长期 TOTP 密钥和本机时间。',
  invalid_totp: 'TOTP 密钥格式或参数不受支持。请使用 Base32 或标准 SHA1 / 六位 / 30 秒的 TOTP URI。',
  malformed: '输入格式不正确，请核对后重试。',
  interaction_required: '自动登录未完成，请根据任务记录核对停止步骤。',
  identity_changed: '登录期间凭证身份已变化，请重新读取并核对目标。',
  policy_changed: '登录期间凭证设置已变化，未覆盖修改。请核对 CPA 状态后重新登录。',
  credential_enable_conflict: '授权文件已保存，但 CPA 尚未确认启用。请核对凭证状态，勿重复登录或导入。',
  duplicate_identity: '该身份已有凭证，请选择已有凭证重新授权。',
  identity_mismatch: '本次登录身份与目标凭证不一致，未更新凭证。',
  identity_unverifiable: '授权未提供足够的身份信息，无法安全导入。',
  workspace_mismatch: '授权工作区与目标凭证不一致。',
  commit_in_progress: '凭证正在提交，请刷新任务核对结果。',
  revision_conflict: '任务版本已变化，请刷新后重新核对。',
  conflict: '状态已变化，请刷新任务并重新预览。',
  queue_full: '登录队列已满，请稍后手动重试。',
  login_budget: '此账号已达到手动登录频率限制，请稍后再试。',
  already_running: '此账号已有登录任务，请查看现有任务。',
  vault_locked: '本机登录资料不可用，请检查存储文件和访问权限。',
  vault_unavailable: '本机登录资料暂不可用，请检查宿主存储状态。',
  vault_not_initialized: '当前宿主尚不支持直接保存登录资料，请升级宿主。',
  vault_save_failed: '授权已提交，但登录资料保存失败。不要重复登录，请核对资料库。',
  store_unavailable: '凭证存储暂不可用，请核对宿主状态。',
  callback_invalid: '回调地址不符合本次授权会话，请复制完整回调地址。',
  state_mismatch: '回调与本次授权会话不一致。',
  expired: '会话已过期，请创建新的登录任务。',
  cancelled: '登录任务已取消。',
  network_error: '认证网络请求失败，未自动重试。',
  authorization_failed: '官方授权未完成，请核对账号状态后手动重试。',
  timeout: '登录等待超时，未自动重试。',
  account_exists: '此账号已保存，请在账号列表中更新资料。',
  duplicate_account: '导入内容含重复账号，或账号已经存在，请核对后再保存。',
  account_limit: '已达到账号数量上限，请先整理不再使用的账号。',
  account_busy: '此账号正在登录或提交，请等待任务结束后再修改。',
  account_unbound: '此账号尚未绑定 CPA 凭证，请先点击 2FA 登录。',
  account_not_linked: '此账号尚未绑定 CPA 凭证，请先点击 2FA 登录。',
  automation_disabled: '自动授权已关闭，本次自动任务不再继续。',
  security_failed: '安全操作未完成，请根据停止步骤核对原因。',
  security_journal_invalid: '本地安全操作记录无法核实。请保留现有记录和备份，勿重复修改账号。',
  security_unavailable: '当前登录引擎不支持安全操作，请更新插件。',
  security_storage_failed: '安全操作记录未能可靠保存，已停止后续步骤。',
  security_outcome_unknown: '安全操作可能已生效，但结果尚未确认；不会自动重试。',
  security_rejected: '官方接口拒绝了本次安全操作。',
  security_state_unconfirmed: '无法确认账号当前的 2FA 状态，已停止修改。',
  security_pending: '此账号有未确认的安全操作。请先查看任务和账号资料中的核对信息，勿重复重置或登录。',
  security_uncertain: '安全操作可能已生效，但结果尚未确认。请查看任务与账号资料中的核对信息，勿重复操作。',
  security_action_unsupported: '当前登录引擎不支持此安全操作，请更新插件后重试。',
  security_identity_mismatch: '安全会话身份与当前账号不一致，已停止操作。',
  security_identity_unverified: '无法核实安全会话对应的账号，未继续修改。',
  security_checkpoint_failed: '安全操作的本地记录未能可靠保存，已停止后续步骤。',
  totp_save_failed: '新 2FA 尚未可靠保存。请查看账号资料中的待核对密钥与任务结果，勿重复重置。',
  totp_factor_ambiguous: '无法确认唯一的旧 2FA 因子，未执行重置。',
  logout_failed: '退出所有设备未完成，请查看任务记录。',

};
const idOf = job => String(job?.id || job?.job_id || '');
const isSecurityJob = job => ['reset_totp', 'logout_all'].includes(job?.action);
const operationLabel = job => job?.action === 'reset_totp' ? '重置 2FA' : job?.action === 'logout_all' ? '退出所有设备' : job?.intent === 'reauthorize' ? '重新授权' : job?.intent === 'import' ? '首次授权' : '登录授权';
const securityResult = job => [job?.totp_reset ? '新 2FA 已保存' : '', job?.devices_logged_out ? '已退出所有设备' : ''].filter(Boolean).join(' · ');
const jobLabel = job => job?.status === 'committed' && job.credential_enabled ? '已保存并启用'
  : job?.status === 'committing' && job.enable_credential ? '保存并启用中'
  : job?.status === 'succeeded' && isSecurityJob(job) ? '安全操作已完成'
  : job?.status === 'running' && isSecurityJob(job) ? '安全操作中'
  : job?.status === 'failed' && isSecurityJob(job) ? securityResult(job) ? '后续步骤未完成' : '安全操作未完成'
  : labels[job?.status] || '待核对';
const key = () => crypto.randomUUID();
const messageFor = code => reasons[code] || '登录操作未完成，请刷新任务核对状态。';
const safeAuthorizationURL = value => {
  try {
    const url = new URL(value);
    return url.protocol === 'https:' && url.hostname === 'auth.openai.com' && !url.port && !url.username && !url.password && url.pathname === '/oauth/authorize' ? url.href : '';
  } catch { return ''; }
};

// Authentication inputs remain in this workbench only until its request is sent.
export function createLoginWorkbench({ request, connected, onCommitted }) {
  const $ = id => document.getElementById(id);
  const state = { open: false, epoch: 0, capabilities: null, targets: [], jobs: [], selected: '', job: null, candidate: null, vault: null, timer: null, busy: new Set(), urls: new Map(), detailSignature: '', storageRead: 0, detailRead: 0, jobsRead: 0, view: 'accounts', accounts: [], accountSettings: null, accountRead: 0, accountsAvailable: false, accountSignature: '', accountEditing: null, accountStarting: '', accountDetailFailed: false, settingsDirty: false, settingsDraftRevision: null, accountDetailRead: 0, accountDetailLoading: false, accountQuery: '', accountFilter: 'all', accountPage: 0, jobSignature: '', revealJob: false };
  const get = path => request(path);
  const write = (path, body, method = 'POST') => request(path, method, body, key());
  const current = epoch => state.open && state.epoch === epoch && connected();
  const methodAvailable = method => state.capabilities?.available !== false && state.capabilities?.methods?.[method]?.available === true;
  const localStorageReady = vault => vault.available !== false && vault.initialized === true && vault.storage_mode === 'plaintext';
  const storageMessage = vault => vault.available === false ? messageFor(vault.reason || 'vault_unavailable') : '当前宿主尚不支持明文保存，请升级宿主后重试。';
  let loading = false;
  const accountLogs = { account: '', scope: 'current', filter: 'all', follow: true, cache: new Map(), read: 0, loading: false, error: '', updated: '', signature: '', row: null };
  let importOpener = null;
  let editorOriginID = '';
  let menuAccountID = '';
  let securityAccount = null;
  let securityAction = '';
  let securitySubmissionUncertain = false;
  let securityRecoveryJob = '';
  let securityRecoveryCompletable = false;

  function importFeedback(message = '') {
    $('account-import-error').textContent = message;
    $('account-import-error').hidden = !message;
  }
  function updateImportCount() {
    const count = $('account-import-text').value.split(/\r?\n/).filter(line => line.trim()).length;
    $('account-import-count').textContent = `${count} / 100 个账号`;
    $('account-import-submit').textContent = state.busy.has('account') && $('account-import-dialog').open ? '正在导入…' : count ? `导入 ${count} 个账号` : '导入账号';
  }
  function closeAccountImport(restoreFocus = false) {
    const opener = importOpener;
    importOpener = null;
    $('account-import-text').value = '';
    $('account-import-proxy').value = '';
    $('account-import-dialog').querySelector('.login-proxy-options').open = false;
    importFeedback();
    if ($('account-import-dialog').open) $('account-import-dialog').close();
    updateImportCount();
    if (restoreFocus && opener?.isConnected && !opener.disabled) opener.focus({ preventScroll: true });
  }

  function feedback(message = '', error = false) {
    if ($('account-security-dialog').open && error) {
      securityFeedback(message);
      $('account-security-error').focus({ preventScroll: true });
      return;
    }
    if ($('account-import-dialog').open && error) {
      importFeedback(message);
      $('account-import-error').focus({ preventScroll: true });
      return;
    }
    if ($('account-editor-dialog').open && error) {
      editorFeedback(message);
      $('account-editor-error').focus({ preventScroll: true });
      return;
    }
    $('login-notice').textContent = message;
    $('login-notice').classList.toggle('hidden', !message);
    $('login-notice').classList.toggle('error', error);
  }
  function fail(error) {
    if (isDiscarded(error)) return;
    const message = error.status === 404 ? '当前宿主尚未提供登录工作台，请安装支持此功能的宿主。'
      : error instanceof TypeError ? '网络中断，提交结果尚未确认。请刷新任务，避免重复登录。'
      : messageFor(error.code);
    feedback(message, true);
  }
  function clearSecrets() {
    $('login-workbench').querySelectorAll('[data-login-secret], #login-callback').forEach(input => { input.value = ''; });
  }
  function setView(view) {
    if (!['accounts', 'tasks', 'settings'].includes(view)) return;
    if (view !== state.view || view === 'accounts' && ($('account-editor-dialog').open || $('account-import-dialog').open)) {
      clearSecrets(); feedback(); closeAccountEditors();
    }
    state.view = view;
    for (const name of ['accounts', 'tasks', 'settings']) $(`login-view-${name}`).classList.toggle('hidden', name !== view);
    document.querySelectorAll('[data-login-view]').forEach(button => {
      const selected = button.dataset.loginView === view;
      button.classList.toggle('active', selected);
      button.setAttribute('aria-current', selected ? 'page' : 'false');
      button.setAttribute('aria-controls', `login-view-${button.dataset.loginView}`);
    });
    document.querySelector('.login-scroll').scrollTop = 0;
    if (view === 'tasks') { state.revealJob = true; revealSelectedJob(); }
    updateControls();
  }
  function editorFeedback(message = '') {
    $('account-editor-error').textContent = message;
    $('account-editor-error').hidden = !message;
  }
  function focusAccount(id, action = 'edit') {
    const button = [...$('account-list').querySelectorAll('[data-account-action]')].find(node => node.dataset.accountId === id && node.dataset.accountAction === action);
    (button && !button.disabled ? button : $('account-import-open')).focus({ preventScroll: true });
  }
  function closeAccountEditor(restoreFocus = false) {
    const id = state.accountEditing?.id || editorOriginID;
    const active = !!id || $('account-editor-dialog').open;
    editorOriginID = '';
    state.accountDetailRead++; state.accountDetailLoading = false; state.accountDetailFailed = false;
    state.accountEditing = null; securityRecoveryJob = ''; securityRecoveryCompletable = false;
    $('account-editor').querySelectorAll('[data-login-secret]').forEach(input => { input.value = ''; });
    $('account-editor-identity').textContent = '';
    $('account-editor-note').textContent = '';
    $('account-label').value = '';
    $('account-delete-confirm').hidden = true;
    $('account-editor-retry').hidden = true;
    $('account-login-options').open = false;
    $('account-login-target').value = '';
    editorFeedback();
    $('account-security-recovery').hidden = true;
    $('account-security-recovery-secret-label').hidden = true;
    $('account-security-recovery-note').textContent = '';
    if ($('account-editor-dialog').open) $('account-editor-dialog').close();
    if (active) renderAccounts();
    if (restoreFocus && id && state.open && connected() && state.view === 'accounts') focusAccount(id);
  }
  function closeAccountEditors() {
    closeAccountImport(); closeAccountEditor(); closeAccountSecurity(); closeAccountMenu();
  }
  function securityFeedback(message = '') {
    $('account-security-error').textContent = message;
    $('account-security-error').hidden = !message;
  }
  function securityAvailable(action) {
    return state.capabilities?.security_actions?.[action] === true && methodAvailable('password_totp');
  }
  function closeAccountMenu(restoreFocus = false) {
    const id = menuAccountID;
    menuAccountID = '';
    if ($('account-actions-menu').matches(':popover-open')) $('account-actions-menu').hidePopover();
    $('account-list').querySelectorAll('[data-account-action="more"]').forEach(button => button.setAttribute('aria-expanded', 'false'));
    if (restoreFocus && id && current(state.epoch) && state.view === 'accounts') focusAccount(id, 'more');
  }
  function showAccountMenu(account, button) {
    if (accountBusy()) return;
    if (menuAccountID === account.id && $('account-actions-menu').matches(':popover-open')) { closeAccountMenu(true); return; }
    closeAccountMenu();
    menuAccountID = account.id;
    updateSecurityControls();
    const menu = $('account-actions-menu');
    menu.showPopover();
    const bounds = button.getBoundingClientRect();
    const width = menu.offsetWidth, height = menu.offsetHeight;
    menu.style.left = `${Math.max(12, Math.min(bounds.right - width, innerWidth - width - 12))}px`;
    menu.style.top = `${Math.max(12, bounds.bottom + height + 8 <= innerHeight ? bounds.bottom + 4 : bounds.top - height - 4)}px`;
    button.setAttribute('aria-expanded', 'true');
    (menu.querySelector('button:not(:disabled)') || menu).focus({ preventScroll: true });
  }
  function updateSecurityControls() {
    const menuAccount = state.accounts.find(account => account.id === menuAccountID);
    const unavailable = accountBusy() || !menuAccount || accountLoginPending(menuAccount);
    $('account-reset-totp').disabled = unavailable || !securityAvailable('reset_totp') || !!menuAccount?.credentials_error || !!menuAccount?.proxy_error;
    $('account-logout-all').disabled = unavailable || !securityAvailable('logout_all') || !!menuAccount?.credentials_error || !!menuAccount?.proxy_error;
    $('account-menu-delete').disabled = unavailable;
    $('account-reset-totp').title = securityAvailable('reset_totp') ? '' : '当前登录引擎不支持重置 2FA';
    $('account-logout-all').title = securityAvailable('logout_all') ? '' : '当前登录引擎不支持退出所有设备';
    const latest = state.accounts.find(account => account.id === securityAccount?.id);
    const busy = state.busy.has('account');
    const blocked = accountBusy() || !latest || accountLoginPending(latest) || latest.revision !== securityAccount?.revision || !securityAvailable(securityAction) || securitySubmissionUncertain;
    $('account-security-submit').disabled = blocked;
    $('account-security-submit').textContent = busy && $('account-security-dialog').open ? '正在提交…' : securityAction === 'logout_all' ? '确认退出所有设备' : '确认重置 2FA';
    $('account-security-form').setAttribute('aria-busy', String(busy));
    $('account-security-logout').disabled = blocked || !securityAvailable('logout_all');
    $('account-security-reauthorize').disabled = blocked;
    for (const id of ['account-security-close', 'account-security-cancel']) $(id).disabled = busy;
    $('account-security-complete').disabled = accountBusy() || state.accountDetailLoading || !securityRecoveryCompletable || !securityRecoveryJob;
    if ($('account-security-dialog').open && !busy && latest && latest.revision !== securityAccount?.revision && !securitySubmissionUncertain) securityFeedback('账号状态已变化。请关闭后重新打开，核对最新状态再操作。');
  }
  function closeAccountSecurity(restoreFocus = false) {
    const id = securityAccount?.id;
    securityAccount = null; securityAction = ''; securitySubmissionUncertain = false;
    $('account-security-identity').textContent = '';
    $('account-security-logout').checked = false;
    $('account-security-reauthorize').checked = true;
    securityFeedback();
    if ($('account-security-dialog').open) $('account-security-dialog').close();
    if (restoreFocus && id && current(state.epoch) && state.view === 'accounts') focusAccount(id, 'more');
  }
  function showAccountSecurity(account, action) {
    if (accountBusy() || accountLoginPending(account) || !securityAvailable(action) || account.credentials_error || account.proxy_error) return;
    closeAccountMenu(); clearSecrets(); closeAccountEditors(); feedback();
    securityAccount = { ...account }; securityAction = action;
    const reset = action === 'reset_totp';
    $('account-security-title').textContent = reset ? '重置 2FA' : '退出所有设备';
    $('account-security-identity').textContent = account.email;
    $('account-security-description').textContent = reset ? '将移除当前 2FA 并生成新密钥。成功后自动更新本机账号资料，旧验证器中的验证码将失效。' : '将退出该账号当前的所有登录设备，包括本次安全会话。其他设备需要重新登录。';
    $('account-security-logout-option').hidden = !reset;
    $('account-security-dialog').showModal();
    updateSecurityControls();
    $('account-security-cancel').focus({ preventScroll: true });
  }
  function renderSecurityRecovery(detail) {
    const recovery = detail.security_recovery;
    const pending = !!detail.account?.security_pending;
    securityRecoveryJob = recovery?.job_id || '';
    securityRecoveryCompletable = !!recovery?.can_complete;
    $('account-security-recovery').hidden = !pending && !recovery;
    $('account-security-recovery-secret-label').hidden = !recovery?.new_totp_secret;
    $('account-security-recovery-secret').value = recovery?.new_totp_secret || '';
    $('account-security-complete').hidden = !securityRecoveryCompletable;
    $('account-security-recovery-note').textContent = securityRecoveryCompletable
      ? '已确认新 2FA 激活，但本地保存尚未完成。点击下方按钮仅完成本机资料保存，不会退出设备、重新授权或上传 CPA。'
      : recovery?.activation_confirmed ? '已确认新 2FA 激活，后续结果仍需核对。当前账号已暂停登录和修改；请结合任务日志处理。'
      : '远端操作结果尚未确认。下方新密钥仅为待核对资料，不代表它已生效。请结合任务日志核对账号，勿再次重置或使用旧密钥重登。';
    if (recovery) for (const name of Object.keys(recovery)) recovery[name] = '';
  }
  function accountLoginPending(account) {
    return !!account.security_pending || activeStates.has(account.last_job_status) || ['awaiting_commit', 'commit_unknown', 'saved_not_adopted', 'security_uncertain'].includes(account.last_job_status);
  }
  function accountBusy() { return loading || !connected() || !state.accountsAvailable || state.busy.has('account'); }
  function updateAccountControls() {
    const busy = accountBusy();
    for (const id of ['account-inspect', 'account-import-open', 'account-import-empty', 'account-settings-open', 'account-settings-save', 'account-default-proxy', 'account-inspection-enabled', 'account-auto-enabled', 'account-interval', 'account-editor-save', 'account-login-start', 'account-delete-yes']) $(id).disabled = busy;
    $('account-inspect').disabled = busy || !state.accounts.length;
    $('account-auto-enabled').disabled = busy || !$('account-inspection-enabled').checked;
    $('account-inspect').setAttribute('aria-busy', String(state.busy.has('account')));
    const vault = state.vault || state.capabilities?.vault || {};
    const storageReady = localStorageReady(vault);
    $('account-import-submit').disabled = busy || !storageReady;
    $('account-import-text').disabled = busy;
    $('account-import-proxy').disabled = busy;
    for (const id of ['account-import-close', 'account-import-cancel']) $(id).disabled = state.busy.has('account');
    $('account-import-form').setAttribute('aria-busy', String(state.busy.has('account')));
    updateImportCount();
    const editing = state.accounts.find(account => account.id === state.accountEditing?.id) || state.accountEditing;
    const editorBusy = busy || state.accountDetailLoading || state.accountDetailFailed || !storageReady || !!editing?.security_pending;
    $('account-editor').setAttribute('aria-busy', String(state.accountDetailLoading || state.busy.has('account')));
    $('account-editor').querySelectorAll('input').forEach(input => { input.disabled = editorBusy; });
    $('account-editor-save').disabled = editorBusy;
    $('account-security-recovery-secret').disabled = busy || state.accountDetailLoading || state.accountDetailFailed;
    for (const id of ['account-editor-close', 'account-editor-cancel', 'account-delete-no']) $(id).disabled = state.busy.has('account');
    $('account-editor-retry').disabled = busy || state.accountDetailLoading;
    $('account-login-target').disabled = editorBusy;
    $('account-login-start').disabled = editorBusy || !editing || accountLoginPending(editing) || !methodAvailable('password_totp') || !!editing.credentials_error || !!editing.proxy_error;
    $('account-delete-open').disabled = editorBusy || !!editing && accountLoginPending(editing);
    $('account-delete-yes').disabled = editorBusy || !!editing && accountLoginPending(editing);
    $('account-replace-secrets').disabled = editorBusy || !state.accountEditing?.email;
    $('account-email').readOnly = true;
    for (const id of ['account-password', 'account-totp']) $(id).readOnly = !$('account-replace-secrets').checked;
    $('account-auto-relogin').disabled = editorBusy || !state.accountEditing?.auth_id;
    $('account-vault-note').textContent = storageReady ? '账号、密码和 2FA 密钥以明文保存在本机。' : storageMessage(vault);
    $('account-list').querySelectorAll('.account-row button').forEach(button => { button.disabled = busy || button.dataset.unavailable === 'true'; });
    const pages = Math.max(1, Math.ceil(filteredAccounts().length / 20));
    updateSecurityControls();
    $('account-page-prev').disabled = busy || state.accountPage === 0;
    $('account-page-next').disabled = busy || state.accountPage >= pages - 1;
  }
  function renderAccountSettings() {
    const settings = state.accountSettings;
    $('account-schedule-summary').textContent = !settings ? '尚未加载' : settings.inspection_enabled ? `每 ${settings.interval_minutes} 分钟 · ${settings.auto_relogin_enabled ? '已允许自动授权' : '只巡检'}` : '定时巡检未开启';
    if (!state.settingsDirty && settings) {
      $('account-inspection-enabled').checked = !!settings.inspection_enabled;
      $('account-auto-enabled').checked = !!settings.auto_relogin_enabled;
      $('account-interval').value = settings.interval_minutes || 15;
      $('account-default-proxy').value = settings.login_proxy_url || '';
    }
  }
  function filteredAccounts() {
    const query = state.accountQuery.trim().toLocaleLowerCase();
    return state.accounts.filter(account => {
      if (query && ![account.email, account.label, account.auth_id].some(value => String(value || '').toLocaleLowerCase().includes(query))) return false;
      if (state.accountFilter === 'linked' && !account.auth_id) return false;
      if (state.accountFilter === 'attention' && !needsAttention(account)) return false;
      return true;
    });
  }
  function renderAccounts() {
    const count = state.accounts.length;
    const filtered = filteredAccounts();
    const pages = Math.max(1, Math.ceil(filtered.length / 20));
    state.accountPage = Math.min(state.accountPage, pages - 1);
    const shown = filtered.slice(state.accountPage * 20, (state.accountPage + 1) * 20);
    const signature = JSON.stringify([shown, state.jobs, state.accountSettings?.login_proxy_url, state.accountEditing?.id, state.accountsAvailable, state.accountStarting, accountLogs.account]);
    $('account-count').textContent = count ? String(count) : '';
    $('account-filter-all-count').textContent = String(count);
    $('account-filter-attention-count').textContent = String(state.accounts.filter(needsAttention).length);
    $('account-filter-linked-count').textContent = String(state.accounts.filter(account => account.auth_id).length);
    $('account-active-count').textContent = String(state.accounts.filter(account => activeStates.has(account.last_job_status)).length);
    $('account-list-summary').textContent = !state.accountsAvailable ? '账号暂未载入，正在重试' : count ? `共 ${count} 个账号` : '导入账号后，在这里管理授权与状态。';
    $('account-list-summary').hidden = state.accountsAvailable;
    $('account-table-wrap').classList.toggle('hidden', !filtered.length || !state.accountsAvailable);
    $('account-empty').classList.toggle('hidden', count > 0 || !state.accountsAvailable);
    $('account-no-results').hidden = !state.accountsAvailable || !count || filtered.length > 0;
    $('account-filter-count').textContent = state.accountQuery || state.accountFilter !== 'all' ? `${filtered.length} 个匹配` : '';
    $('account-page-label').textContent = filtered.length ? `${state.accountPage * 20 + 1}–${Math.min((state.accountPage + 1) * 20, filtered.length)} / ${filtered.length}` : '0 个账号';
    document.querySelector('.account-pagination').hidden = !count || pages <= 1 && !state.accountQuery && state.accountFilter === 'all';
    $('account-page-prev').hidden = pages <= 1;
    $('account-page-next').hidden = pages <= 1;
    document.querySelectorAll('[data-account-filter]').forEach(button => {
      const selected = button.dataset.accountFilter === state.accountFilter;
      button.classList.toggle('active', selected); button.setAttribute('aria-pressed', String(selected));
    });
    if (signature !== state.accountSignature) {
      const activeElement = document.activeElement;
      const logFocused = accountLogs.row?.contains(activeElement) ? activeElement : null;
      const logFocusedKey = logFocused?.closest('details')?.dataset.logKey;
      const logTop = accountLogs.row?.querySelector('#account-log-viewport')?.scrollTop || 0;
      const focused = activeElement?.dataset;
      state.accountSignature = signature;
      accountLogs.row?.remove();
      $('account-list').innerHTML = shown.map(account => {
        const id = escapeHTML(account.id);
        const job = state.jobs.find(item => idOf(item) === account.last_job_id) || { status: account.last_job_status, error_code: account.last_job_error_code, error_stage: account.last_job_error_stage, error_reason: account.last_job_error_reason, auto_commit: account.last_job_auto_commit, enable_credential: account.last_job_auto_commit, credential_enabled: account.last_job_credential_enabled, action: account.last_job_action, security_status: account.last_job_security_status, totp_reset: account.last_job_totp_reset, devices_logged_out: account.last_job_devices_logged_out };
        const failed = ['failed', 'interaction_required', 'conflict', 'interrupted', 'expired'].includes(account.last_job_status);
        const issue = job.status === 'interaction_required' && !job.error_reason ? '未记录具体原因' : jobDiagnostic({ ...job, error_stage: '' });
        const healthDetail = account.health_reason && !['no_auth_failure_observed', 'unbound', 'login_required', 'interaction_required'].includes(account.health_reason) ? healthReason(account.health_reason) : '';
        const last = account.last_job_status ? jobLabel({ ...job, status: account.last_job_status }) : '尚未登录';
        const unresolved = ['commit_unknown', 'saved_not_adopted', 'security_uncertain'].includes(account.last_job_status);
        const active = accountLoginPending(account);
        const submitting = state.accountStarting === account.id;
        const auto = account.auto_relogin ? (account.attention_required ? '自动授权已暂停' : '允许自动授权') : '手动授权';
        const healthTone = ['needs_reauth', 'requires_sign_in', 'auth_failed', 'missing', 'identity_changed'].includes(account.health_status) ? 'bad' : ['paused', 'quality_paused', 'disabled', 'manual_disabled', 'cooldown', 'native_cooldown'].includes(account.health_status) ? 'warn' : '';
        const jobTone = (account.last_job_status === 'committed' && job.credential_enabled || account.last_job_status === 'succeeded') ? 'good' : account.last_job_status === 'interaction_required' ? 'warn' : failed ? 'bad' : unresolved || account.last_job_status === 'awaiting_commit' ? 'warn' : '';
        const loginLabel = submitting ? '正在提交…' : account.security_pending && !activeStates.has(account.last_job_status) ? '待核对' : unresolved ? '等待核对' : account.last_job_status === 'awaiting_commit' ? '待确认' : account.last_job_status === 'committing' ? '保存中…' : active ? last : account.auth_id ? '重新登录' : '2FA 登录';
        return `<tr class="account-row${state.accountEditing?.id === account.id || accountLogs.account === account.id ? ' selected' : ''}" data-account-row="${id}">
          <td data-label="账号"><div class="account-identity">
            <strong class="account-email">${escapeHTML(account.email || '邮箱暂不可读取')}</strong>
            ${account.label ? `<span class="account-label">${escapeHTML(account.label)}</span>` : ''}
            <small class="account-automation"><span class="account-method">密码与 2FA</span> ${escapeHTML(auto)}</small>
            ${account.credentials_error ? '<small class="error-note">登录资料暂不可读取</small>' : ''}
          </div></td>
          <td data-label="CPA 状态"><div class="account-status">
            <span class="tag ${healthTone}">${escapeHTML(healthLabel(account.health_status))}</span>
            ${account.auth_id ? `<small class="account-binding">${escapeHTML(account.auth_id)}</small>` : ''}
            ${healthDetail ? `<small class="account-status-reason">${escapeHTML(healthDetail)}</small>` : ''}
            <small class="account-inspection">${account.last_checked_at ? `巡检 ${escapeHTML(compactDate(account.last_checked_at))}` : '尚未巡检'}${account.next_check_at ? ` · 下次 ${escapeHTML(compactDate(account.next_check_at))}` : ''}</small>
          </div></td>
          <td data-label="当前操作"><div class="account-activity">
            <div class="account-activity-heading"><span class="account-job-status ${jobTone}">${escapeHTML(last)}</span>${job.error_stage ? `<small class="account-operation-stage">${escapeHTML(diagnosticStage(job.error_stage))}</small>` : ''}</div>
            ${issue ? `<small class="account-status-reason">${escapeHTML(issue)}</small>` : active && job.phase ? `<small>${escapeHTML(phaseLabel(job.phase))}</small>` : ''}
            ${securityResult(job) ? `<small class="account-security-result">${escapeHTML(securityResult(job))}</small>` : ''}
            ${account.security_pending && !activeStates.has(account.last_job_status) ? '<small class="account-status-reason">安全操作尚未确认，请查看资料与任务。</small>' : ''}
            ${account.proxy_error ? `<small class="account-proxy-error error-note">${escapeHTML(messageFor(account.proxy_error))}</small>` : ''}
            ${account.last_job_status === 'awaiting_commit' ? `<button type="button" class="text-button small" data-account-action="task" data-account-id="${id}">核对授权</button>` : ''}
          </div></td>
          <td data-label="操作时间"><div class="account-times">
            <span><small>操作</small><time>${job.created_at ? escapeHTML(compactDate(job.created_at)) : '—'}</time></span>
            <span><small>授权</small><time>${account.last_login_at ? escapeHTML(compactDate(account.last_login_at)) : '—'}</time></span>
          </div></td>
          <td data-label="操作类型"><button type="button" class="account-operation-type" data-account-action="logs" data-account-id="${id}" aria-expanded="${accountLogs.account === account.id}"><strong>${account.last_job_id ? operationLabel(job) : '尚无操作'}</strong><small>${account.last_job_id ? job.automatic ? '自动执行' : '手动执行' : '查看日志'}</small></button></td>
          <td data-label="操作"><div class="account-row-actions">
            <button type="button" class="subtle small" data-account-action="start" data-account-id="${id}" title="登录成功后自动保存并启用 CPA 凭证" data-unavailable="${active || !methodAvailable('password_totp') || !!account.credentials_error || !!account.proxy_error}" ${active ? 'disabled' : ''}>${accountIcon('login')}${escapeHTML(loginLabel)}</button>
            <button type="button" class="subtle small" data-account-action="edit" data-account-id="${id}">${accountIcon('user')}资料</button>
            <button type="button" class="subtle small" data-account-action="more" data-account-id="${id}" aria-controls="account-actions-menu" aria-expanded="${menuAccountID === account.id}" aria-label="${escapeHTML(account.email || '账号')}的更多操作">${accountIcon('more')}更多</button>
            <button type="button" class="subtle small" data-account-action="task" data-account-id="${id}" data-unavailable="${!account.last_job_id}">${accountIcon('task')}${account.last_job_status === 'awaiting_commit' ? '核对授权' : '任务'}</button>
            <button type="button" class="subtle small" data-account-action="logs" data-account-id="${id}" aria-expanded="${accountLogs.account === account.id}">${accountIcon('logs')}日志</button>
            <button type="button" class="subtle small" data-account-action="inspect" data-account-id="${id}">${accountIcon('refresh')}巡检</button>
          </div></td>
        </tr>`;
      }).join('');
      mountAccountLogs();
      if (accountLogs.row) accountLogs.row.querySelector('#account-log-viewport').scrollTop = logTop;
      if (logFocused?.isConnected) logFocused.focus({ preventScroll: true });
      else if (logFocusedKey && accountLogs.row) [...accountLogs.row.querySelectorAll('details')].find(detail => detail.dataset.logKey === logFocusedKey)?.querySelector('summary').focus({ preventScroll: true });
      if (focused?.accountId && focused?.accountAction) [...$('account-list').querySelectorAll('[data-account-action]')].find(button => button.dataset.accountId === focused.accountId && button.dataset.accountAction === focused.accountAction && button.className === activeElement.className)?.focus({ preventScroll: true });
    }
    updateAccountControls();
  }
  function jobsForAccount() {
    const account = state.accounts.find(item => item.id === accountLogs.account);
    if (!account) return [];
    const jobs = state.jobs.filter(job => job.account_id === account.id || idOf(job) === account.last_job_id);
    jobs.sort((a, b) => String(b.created_at || '').localeCompare(String(a.created_at || '')));
    if (accountLogs.scope === 'history') return jobs.reverse();
    return jobs.filter(job => idOf(job) === account.last_job_id).slice(0, 1).concat(jobs.length && !jobs.some(job => idOf(job) === account.last_job_id) ? [jobs[0]] : []);
  }
  function resetAccountLogs() {
    accountLogs.read++;
    accountLogs.row?.remove();
    Object.assign(accountLogs, { account: '', scope: 'current', filter: 'all', follow: true, loading: false, error: '', updated: '', signature: '', row: null });
    accountLogs.cache.clear();
  }
  function toggleAccountLogs(account) {
    const close = accountLogs.account === account.id;
    resetAccountLogs();
    if (!close) accountLogs.account = account.id;
    renderAccounts();
    if (!close) loadAccountLogs();
  }
  function mountAccountLogs() {
    const anchor = [...$('account-list').querySelectorAll('.account-row')].find(row => row.dataset.accountRow === accountLogs.account);
    if (!anchor) return;
    if (!accountLogs.row) {
      const row = document.createElement('tr');
      row.className = 'account-log-row';
      row.innerHTML = `<td colspan="6"><section class="account-log-panel" aria-label="账号操作日志">
        <div class="account-log-header"><div class="account-log-heading">${accountIcon('logs')}<strong>操作日志</strong><span data-log-count>0 条</span></div>
          <div class="account-log-tools"><div class="account-log-filters" aria-label="日志范围"><button type="button" data-account-log-action="current">本次</button><button type="button" data-account-log-action="history">历史记录</button></div>
          <div class="account-log-filters" aria-label="日志筛选"><button type="button" data-account-log-action="all">全部</button><button type="button" data-account-log-action="errors">异常 <span data-log-errors>0</span></button></div>
          <button type="button" class="account-log-tool" data-account-log-action="refresh" aria-label="刷新操作日志" title="刷新">${accountIcon('refresh')}</button><button type="button" class="account-log-tool" data-account-log-action="copy" aria-label="复制当前日志" title="复制当前显示日志">${accountIcon('copy')}</button></div>
        </div><div class="account-log-read-error" role="status" hidden></div><div class="account-log-result" aria-label="本次登录结果" hidden></div>
        <div class="account-log-column-head"><span>时间</span><span>类别</span><span>执行内容</span></div>
        <div id="account-log-viewport" class="account-log-viewport" tabindex="0" role="region" aria-label="操作日志内容"></div>
        <div class="account-log-footer"><span data-log-update role="status"></span><button type="button" data-account-log-action="follow" aria-pressed="true">跟随最新</button></div>
      </section></td>`;
      accountLogs.row = row;
      row.addEventListener('toggle', event => {
        if (event.target.matches('details[open]') && accountLogs.follow) { accountLogs.follow = false; renderAccountLogs(); }
      }, true);
      row.querySelector('#account-log-viewport').addEventListener('scroll', event => {
        const viewport = event.currentTarget;
        if (viewport.scrollHeight - viewport.clientHeight - viewport.scrollTop > 12 && accountLogs.follow) {
          accountLogs.follow = false; renderAccountLogs();
        }
      });
    }
    anchor.after(accountLogs.row);
    renderAccountLogs();
  }
  function visibleLogEntries() {
    return jobsForAccount().flatMap(job => (accountLogs.cache.get(idOf(job))?.entries || []).map(entry => ({ ...entry, job_id: idOf(job) })))
      .filter(entry => accountLogs.filter !== 'errors' || entry.level === 'error' || entry.level === 'warn');
  }
  function renderAccountLogs() {
    const row = accountLogs.row;
    if (!row) return;
    const jobs = jobsForAccount();
    const entries = visibleLogEntries();
    const all = jobs.flatMap(job => accountLogs.cache.get(idOf(job))?.entries || []);
    row.querySelector('[data-log-count]').textContent = `${all.length} 条`;
    row.querySelector('[data-log-errors]').textContent = String(all.filter(entry => ['warn', 'error'].includes(entry.level)).length);
    for (const action of ['current', 'history', 'all', 'errors', 'follow']) row.querySelector(`[data-account-log-action="${action}"]`).setAttribute('aria-pressed', String([accountLogs.scope, accountLogs.filter, accountLogs.follow && 'follow'].includes(action)));
    row.querySelector('[data-account-log-action="refresh"]').disabled = accountLogs.loading;
    row.querySelector('[data-account-log-action="copy"]').disabled = !entries.length;
    const error = row.querySelector('.account-log-read-error');
    error.hidden = !accountLogs.error; error.textContent = accountLogs.error;
    row.querySelector('[data-log-update]').textContent = `${accountLogs.loading ? '正在读取…' : accountLogs.updated ? `更新于 ${new Date(accountLogs.updated).toLocaleTimeString('zh-CN', { hour12: false })}` : '尚未读取'}${accountLogs.scope === 'history' ? ' · 最近 200 条任务范围，每次操作最多保留 200 条日志' : ' · 本次操作最多保留 200 条日志'}`;
    const result = row.querySelector('.account-log-result');
    const latest = all.at(-1);
    result.hidden = accountLogs.scope !== 'current' || !latest || !['warn', 'error'].includes(latest.level);
    if (!result.hidden) {
      const summary = logEntrySummary(latest, jobs.at(-1));
      const duration = all[0]?.id === 1 && all.length > 1 ? (Date.parse(latest.time) - Date.parse(all[0].time)) / 1000 : NaN;
      result.classList.toggle('is-error', latest.level === 'error');
      result.innerHTML = `<div><strong>${escapeHTML(summary.title)}</strong>${Number.isFinite(duration) && duration >= 0 ? `<span>本次耗时 ${duration.toFixed(2)} 秒</span>` : ''}</div><p>${escapeHTML(summary.detail)}</p><small>${escapeHTML(diagnosticAdvice({ ...jobs.at(-1), ...latest }))}</small>`;
    }
    const signature = JSON.stringify([jobs.map(job => [idOf(job), job.status]), entries, jobs.map(job => accountLogs.cache.get(idOf(job))?.legacy), accountLogs.filter, accountLogs.scope, accountLogs.loading && !all.length]);
    if (signature === accountLogs.signature) return;
    accountLogs.signature = signature;
    const viewport = row.querySelector('#account-log-viewport');
    const top = viewport.scrollTop;
    const opened = new Set([...viewport.querySelectorAll('details[open]')].map(detail => detail.dataset.logKey));
    const focused = document.activeElement?.closest('details')?.dataset.logKey;
    viewport.innerHTML = jobs.map(job => {
      const saved = accountLogs.cache.get(idOf(job));
      const list = entries.filter(entry => entry.job_id === idOf(job));
      const group = accountLogs.scope === 'history' || saved?.legacy ? `<div class="account-log-operation-group"><strong>${escapeHTML(operationLabel(job))}</strong><time>${escapeHTML(compactDate(job.created_at))}</time><span>${escapeHTML(jobLabel(job))}</span><code title="${escapeHTML(idOf(job))}">任务 ${escapeHTML(idOf(job))}</code></div>` : '';
      const legacy = saved?.legacy ? '<p class="account-log-legacy">此任务创建于日志功能上线前，没有记录早期步骤日志。当前任务状态见上方。</p>' : '';
      return group + legacy + list.map(entry => {
        const entryKey = `${idOf(job)}:${entry.id}`;
        const summary = logEntrySummary(entry, job);
        return `<div class="account-log-entry ${entry.level === 'error' ? 'is-error' : entry.level === 'warn' ? 'is-warning' : ''}"><time title="${escapeHTML(date(entry.time))}">${escapeHTML(logTime(entry.time))}</time><span class="account-log-kind">${entry.level === 'error' ? '错误' : entry.level === 'warn' ? '提醒' : ['committed', 'succeeded'].includes(entry.status) ? '完成' : '进度'}</span><details data-log-key="${escapeHTML(entryKey)}"${opened.has(entryKey) ? ' open' : ''}><summary>${accountIcon('chevron')}<span class="account-log-message"><strong class="account-log-title">${escapeHTML(summary.title)}</strong>${summary.detail ? `<span class="account-log-detail">${escapeHTML(summary.detail)}</span>` : ''}</span></summary><pre>${escapeHTML(JSON.stringify(entry, null, 2))}</pre></details></div>`;
      }).join('');
    }).join('') + (!entries.length ? `<p class="account-log-empty">${accountLogs.loading ? '正在读取操作日志…' : accountLogs.filter === 'errors' ? '暂无异常日志' : '暂无操作日志'}</p>` : '');
    if (focused) [...viewport.querySelectorAll('details')].find(detail => detail.dataset.logKey === focused)?.querySelector('summary').focus({ preventScroll: true });
    viewport.scrollTop = accountLogs.follow ? viewport.scrollHeight : top;
  }
  async function loadAccountLogs(force = false) {
    if (!accountLogs.account || !state.open || !connected()) return;
    const accountID = accountLogs.account;
    const epoch = state.epoch;
    const sequence = ++accountLogs.read;
    const valid = () => current(epoch) && accountID === accountLogs.account && sequence === accountLogs.read;
    const jobs = jobsForAccount();
    accountLogs.loading = true; accountLogs.error = ''; renderAccountLogs();
    try {
      const pending = jobs.filter(job => force || accountLogs.cache.get(idOf(job))?.revision !== job.revision || !accountLogs.cache.has(idOf(job)) || activeStates.has(job.status));
      for (let index = 0; index < pending.length; index += 4) {
        const responses = await Promise.allSettled(pending.slice(index, index + 4).map(async job => {
          const response = await get(`/logs?id=${encodeURIComponent(idOf(job))}`);
          if (response.job_id !== idOf(job) || !Array.isArray(response.entries)) throw new Error('invalid_log_response');
          return { job, response };
        }));
        if (!valid()) return;
        for (const response of responses) {
          if (response.status === 'rejected') { accountLogs.error = '日志读取未完成，已保留已加载内容。请点击刷新重试。'; continue; }
          const { job, response: data } = response.value;
          // Keep only the documented event fields; upstream payloads are never log content.
          const entries = data.entries.slice(-200).map(entry => ({ id: Number(entry.id), time: String(entry.time || ''), level: ['info', 'warn', 'error'].includes(entry.level) ? entry.level : 'info', status: String(entry.status || ''), phase: String(entry.phase || ''), ...(entry.error_code ? { error_code: String(entry.error_code) } : {}), ...(entry.error_stage ? { error_stage: String(entry.error_stage) } : {}), ...(entry.error_reason ? { error_reason: String(entry.error_reason) } : {}) }));
          accountLogs.cache.set(idOf(job), { entries, legacy: !!data.legacy, revision: job.revision });
        }
      }
      if (valid() && !accountLogs.error) accountLogs.updated = new Date().toISOString();
    } finally {
      if (valid()) { accountLogs.loading = false; renderAccountLogs(); }
    }
  }
  $('account-list').addEventListener('click', async event => {
    const button = event.target.closest('[data-account-log-action]');
    if (!button || button.disabled) return;
    const action = button.dataset.accountLogAction;
    if (action === 'current' || action === 'history') { accountLogs.scope = action; renderAccountLogs(); await loadAccountLogs(); }
    else if (action === 'all' || action === 'errors') { accountLogs.filter = action; renderAccountLogs(); }
    else if (action === 'refresh') await loadAccountLogs(true);
    else if (action === 'follow') {
      accountLogs.follow = !accountLogs.follow; renderAccountLogs();
      if (accountLogs.follow) { const viewport = $('account-log-viewport'); viewport.scrollTop = viewport.scrollHeight; }
    } else if (action === 'copy') {
      const accountID = accountLogs.account;
      try {
        await navigator.clipboard.writeText(visibleLogEntries().map(entry => JSON.stringify(entry)).join('\n'));
        if (accountID === accountLogs.account) accountLogs.row.querySelector('[data-log-update]').textContent = '已复制当前显示的日志';
      } catch {
        if (accountID === accountLogs.account) { accountLogs.error = '复制失败，请允许剪贴板访问后重试。'; renderAccountLogs(); }
      }
    }
  });

  async function loadAccounts() {
    const epoch = state.epoch;
    const sequence = ++state.accountRead;
    try {
      const [accounts, settings] = await Promise.all([get('/accounts'), get('/accounts/settings')]);
      if (!current(epoch) || sequence !== state.accountRead) return;
      state.accounts = Array.isArray(accounts.items) ? accounts.items : [];
      state.accountSettings = settings;
      state.accountsAvailable = true;
      renderAccountSettings(); renderAccounts();
    } catch (error) {
      if (!current(epoch) || sequence !== state.accountRead || isDiscarded(error)) return;
      state.accountsAvailable = false;
      renderAccounts();
      if (error.status !== 404) feedback('账号数据暂未读取，将自动重试。请检查连接或稍后刷新。', true);
    }
  }
  async function accountAction(action) {
    if (accountBusy()) return;
    const epoch = state.epoch;
    state.busy.add('account'); updateAccountControls(); feedback(); editorFeedback();
    try { await action(epoch); }
    catch (error) { if (current(epoch)) { fail(error); try { await loadAccounts(); } catch {} } }
    finally { state.busy.delete('account'); if (current(epoch)) updateAccountControls(); }
  }
  function showAccountImport(event) {
    if (accountBusy()) return;
    clearSecrets(); closeAccountEditors(); feedback();
    importOpener = event?.currentTarget || $('account-import-open');
    updateAccountControls();
    $('account-import-dialog').showModal();
    $('account-import-text').focus({ preventScroll: true });
  }
  async function showAccountEditor(account) {
    closeAccountMenu();
    if (accountBusy()) return;
    clearSecrets(); closeAccountEditors(); feedback();
    editorOriginID = account.id;
    const epoch = state.epoch;
    const sequence = ++state.accountDetailRead;
    state.accountEditing = { ...account };
    state.accountDetailLoading = true; state.accountDetailFailed = false;
    editorFeedback();
    $('account-editor-retry').hidden = true;
    $('account-editor-identity').textContent = account.email || '';
    $('account-login-options').hidden = !!account.auth_id;
    $('account-login-target').innerHTML = '<option value="">创建新的 CPA 凭证</option>' + state.targets.filter(target => target.can_update !== false).map(target => `<option value="${escapeHTML(target.auth_id)}">更新 ${escapeHTML(target.email ? `${target.email} · ${target.auth_id}` : target.auth_id)}</option>`).join('');
    $('account-editor-title').textContent = '账号资料';
    $('account-label').value = account.label || '';
    $('account-proxy-url').value = account.proxy_url || '';
    $('account-email').value = account.email || '';
    $('account-auto-relogin').checked = !!account.auto_relogin;
    $('account-replace-secrets').checked = false;
    $('account-secret-editor').hidden = false;
    $('account-secret-editor').querySelectorAll('input').forEach(input => { input.required = false; });
    $('account-security-recovery').hidden = true;
    $('account-security-recovery-secret').value = '';
    $('account-editor-note').textContent = '正在读取登录资料…';
    renderAccounts();
    $('account-editor-dialog').showModal();
    $('account-editor-close').focus({ preventScroll: true });
    try {
      const detail = await get(`/accounts/detail?id=${encodeURIComponent(account.id)}`);
      if (!current(epoch) || sequence !== state.accountDetailRead || state.view !== 'accounts' || state.accountEditing?.id !== account.id) {
        if (detail.security_recovery) for (const name of Object.keys(detail.security_recovery)) detail.security_recovery[name] = '';
        if (detail.credentials) for (const name of Object.keys(detail.credentials)) detail.credentials[name] = '';
        return;
      }
      state.accountEditing = { ...detail.account };
      $('account-label').value = detail.account.label || '';
      $('account-proxy-url').value = detail.account.proxy_url || '';
      $('account-auto-relogin').checked = !!detail.account.auto_relogin;
      $('account-email').value = detail.credentials.email || '';
      $('account-password').value = detail.credentials.password || '';
      $('account-totp').value = detail.credentials.totp_secret || '';
      for (const name of Object.keys(detail.credentials)) detail.credentials[name] = '';
      renderSecurityRecovery(detail);
      $('account-editor-note').textContent = detail.account.auth_id ? `CPA 凭证：${detail.account.auth_id}` : '尚未绑定 CPA 凭证。完成首次登录后可开启自动授权。';
    } catch (error) {
      if (current(epoch) && sequence === state.accountDetailRead) {
        $('account-email').value = account.email || '';
        state.accountDetailFailed = true;
        $('account-editor-note').textContent = '';
        editorFeedback(`${error instanceof TypeError ? '登录资料读取失败。' : messageFor(error.code)} 可点击重试重新读取。`);
        $('account-editor-retry').hidden = false;
      }
    } finally {
      if (current(epoch) && sequence === state.accountDetailRead) { state.accountDetailLoading = false; updateAccountControls(); }
    }
  }
  async function startAccount(account, targetID = '') {
    if (accountBusy() || accountLoginPending(account) || !methodAvailable('password_totp') || account.credentials_error || account.proxy_error) return;
    const epoch = state.epoch;
    const fromEditor = $('account-editor-dialog').open;
    state.accountStarting = account.id;
    renderAccounts();
    await accountAction(async requestEpoch => {
      const result = await write('/accounts/start', { id: account.id, revision: account.revision, ...(targetID ? { target_auth_id: targetID } : {}) });
      if (!current(requestEpoch)) return;
      const job = result.job || result;
      const saved = state.accounts.find(item => item.id === account.id);
      if (saved) Object.assign(saved, { last_job_id: idOf(job), last_job_status: job.status || 'queued', last_job_auto_commit: !!job.auto_commit, last_job_credential_enabled: !!job.credential_enabled });
      state.jobs = [job, ...state.jobs.filter(item => idOf(item) !== idOf(job))];
      if (state.view === 'accounts') selectJob(idOf(job));
      if (fromEditor) closeAccountEditor();
      renderAccounts();
      feedback(job.auto_commit ? '已开始 2FA 登录，成功后自动保存并启用 CPA 凭证。进度会在账号行更新。' : '已开始 2FA 登录，进度会在账号行更新。');
      await reload();
    });
    if (current(epoch) && state.accountStarting === account.id) {
      state.accountStarting = '';
      renderAccounts();
      if (fromEditor && !$('account-editor-dialog').open && state.view === 'accounts') focusAccount(account.id);
    }
  }
  function updateControls() {
    const runtime = state.capabilities?.runtime;
    $('login-engine').hidden = !runtime?.engine;
    $('login-engine').textContent = runtime?.engine === 'go_sobek_protocol' ? `Go + Sobek · ${runtime.runtime_version}` : runtime?.engine === 'official_browser_forms' ? `浏览器引擎 · ${runtime.runtime_version}` : '';
    const unavailable = !state.open || !connected() || loading;
    $('login-refresh').disabled = unavailable;
    $('login-export').disabled = unavailable || !state.jobs.length;
    $('login-job-detail').querySelectorAll('[data-login-action]').forEach(button => { button.disabled = unavailable || state.busy.has('action'); });
    updateAccountControls();
  }
  async function loadStorageStatus() {
    const sequence = ++state.storageRead;
    const epoch = state.epoch;
    state.vault = state.capabilities?.vault || null;
    updateControls();
    try {
      const result = await get('/vault/status');
      if (!current(epoch) || sequence !== state.storageRead) return;
      state.vault = result;
      updateControls();
    } catch (error) { if (current(epoch) && !isDiscarded(error)) updateControls(); }
  }
  function renderJobs() {
    const signature = JSON.stringify([state.jobs, state.selected]);
    if (state.jobSignature === signature) { revealSelectedJob(); return; }
    state.jobSignature = signature;
    $('login-job-count').textContent = state.jobs.length ? String(state.jobs.length) : '';
    const focused = document.activeElement?.dataset.loginJob;
    $('login-jobs').innerHTML = state.jobs.length ? state.jobs.map(job => {
      const id = idOf(job);
      const status = jobLabel(job);
      return `<button type="button" class="login-job${state.selected === id ? ' selected' : ''}" data-login-job="${escapeHTML(id)}" aria-pressed="${state.selected === id}"><span><strong>${escapeHTML(job.email || (job.method === 'browser' ? '浏览器授权' : '本地账号登录'))}</strong><small>${escapeHTML(date(job.created_at))} · ${job.automatic ? '自动执行' : '手动执行'} · ${escapeHTML(operationLabel(job))}</small></span><span class="tag ${['failed','conflict','commit_unknown'].includes(job.status) ? 'bad' : job.status === 'security_uncertain' ? 'warn' : ['committed','succeeded'].includes(job.status) ? 'good' : ''}">${escapeHTML(status)}</span></button>`;
    }).join('') : '<p class="empty-line">暂无登录任务。开始登录后在这里查看进度。</p>';
    if (focused) [...$('login-jobs').querySelectorAll('[data-login-job]')].find(button => button.dataset.loginJob === focused)?.focus({ preventScroll: true });
    $('login-export').disabled = !state.jobs.length;
    revealSelectedJob();
  }
  function revealSelectedJob() {
    if (!state.revealJob || state.view !== 'tasks') return;
    const list = $('login-jobs');
    const selected = list.querySelector('.login-job.selected');
    if (!selected || selected.dataset.loginJob !== state.selected || !list.clientHeight) return;
    const bounds = list.getBoundingClientRect();
    const row = selected.getBoundingClientRect();
    const top = bounds.top + list.clientTop;
    const bottom = top + list.clientHeight;
    if (row.top < top || row.height > list.clientHeight) list.scrollTop += row.top - top;
    else if (row.bottom > bottom) list.scrollTop += row.bottom - bottom;
    state.revealJob = false;
  }
  function renderDetail(force = false) {
    const job = state.job;
    if (!job) return;
    const candidate = state.candidate;
    const signature = JSON.stringify([idOf(job), job.email, job.status, job.phase, job.error_code, job.error_stage, job.error_reason, job.vault_error, job.auto_commit, job.enable_credential, job.credential_enabled, job.action, job.security_status, job.totp_reset, job.devices_logged_out, job.reauthorize, candidate, state.urls.get(idOf(job))]);
    if (!force && state.detailSignature === signature) return;
    state.detailSignature = signature;
    const id = idOf(job);
    const url = safeAuthorizationURL(job.authorization_url || state.urls.get(id));
    const awaiting = job.status === 'awaiting_commit';
    const restrictions = Array.isArray(candidate?.restrictions) ? candidate.restrictions : [];
    const heading = `<div class="heading-row"><h3>${escapeHTML(jobLabel(job))}</h3><small>${escapeHTML(isSecurityJob(job) ? operationLabel(job) : job.automatic ? '自动重新授权' : '手动登录')} · ${job.method === 'browser' ? '浏览器授权' : '密码与 2FA'}</small></div><p class="field-note">${escapeHTML(job.email || '等待身份核验')}</p>`;
    let body = '';
    if (job.error_code) body += `<div class="login-diagnostic"><p class="field-error">${escapeHTML(jobDiagnostic(job) || (job.automatic ? automaticError(job.error_code) : messageFor(job.error_code)))}</p><p class="field-note">${escapeHTML(diagnosticAdvice(job))}</p><dl class="login-diagnostic-facts"><div><dt>错误代码</dt><dd>${escapeHTML(job.error_code)}</dd></div>${job.error_stage ? `<div><dt>停止步骤</dt><dd>${escapeHTML(diagnosticStage(job.error_stage))}</dd></div>` : ''}</dl></div>`;
    if (securityResult(job)) body += `<p class="account-security-result">${escapeHTML(securityResult(job))}。${job.status === 'committed' ? '' : job.reauthorize ? '后续授权结果请结合当前任务状态核对。' : '本次未选择重新授权 CPA。'}</p>`;
    if (job.status === 'security_uncertain') body += '<div class="notice error">安全操作结果尚未确认，账号已阻止再次登录或修改。请在账号资料中查看保留的新密钥和任务信息，勿重复重置或退出。</div>';
    if (job.status === 'succeeded') body += '<p class="login-completed">所选安全操作已完成。</p>';
    if (job.vault_error) body += '<p class="field-error">授权提交已完成，但登录资料未保存。请核对凭证状态后单独保存资料，无需重复登录。</p>';
    if (job.status === 'waiting_browser' && job.method === 'browser') body += `<div class="login-browser-action">${url ? `<a class="login-auth-link" href="${escapeHTML(url)}" target="_blank" rel="noopener noreferrer">打开 OpenAI 官方授权页面<svg class="nav-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M14 4h6v6M20 4 10 14M10 4H4v16h16v-6"/></svg></a>` : '<p class="field-note">授权链接暂不可用，请刷新此任务。</p>'}<p class="field-note">完成官方登录后，复制地址栏中的完整回调地址。即使回调页面无法打开，也可在这里提交。</p><label for="login-callback">回调地址<input id="login-callback" type="text" autocomplete="off" maxlength="8192" placeholder="http://localhost:1455/auth/callback?…"></label><button type="button" class="subtle small" data-login-action="callback">提交回调并核验</button></div>`;
    if (awaiting && candidate) body += `<div class="login-candidate"><h4>核对候选凭证</h4><dl class="evidence-facts"><div><dt>账号</dt><dd>${escapeHTML(candidate.email || '未提供')}</dd></div><div><dt>账号／工作区</dt><dd>${escapeHTML(candidate.account_id || '未提供')}</dd></div><div><dt>用户 ID</dt><dd>${escapeHTML(candidate.user_id || '未提供')}</dd></div><div><dt>提交动作</dt><dd>${candidate.intent === 'reauthorize' || job.intent === 'reauthorize' ? '更新已有凭证的授权' : '创建默认人工禁用的新凭证'}</dd></div><div><dt>目标凭证</dt><dd>${escapeHTML(candidate.target_auth_id || '导入时创建')}</dd></div></dl>${restrictions.length ? `<ul class="restriction-list">${restrictions.map(value => `<li>${escapeHTML(reasonLabel(value))}</li>`).join('')}</ul>` : ''}<p class="login-policy-note">提交前会重新核对已有凭证并保留其设置。确认期间请勿在其他页面同时修改该凭证。</p><div class="button-row login-candidate-actions"><button type="button" class="primary" data-login-action="commit">${job.intent === 'reauthorize' ? '确认更新授权' : '确认导入凭证'}</button><button type="button" class="subtle" data-login-action="discard">丢弃候选</button></div></div>`;
    else if (awaiting) body += '<p class="field-note">正在读取候选身份…</p>';
    if (job.status === 'committed') body += `<p class="login-completed">${job.credential_enabled ? '凭证已保存到 CPA 并启用，无需再次确认。' : '授权已保存到 CPA。'}</p><p class="field-note">相关检测规则保持停止。此操作未调用模型。</p>`;
    if (['saved_not_adopted', 'commit_unknown'].includes(job.status)) body += '<div class="notice error">提交尚未完整确认。请刷新任务并由宿主管理员核对，勿重新登录或重复导入。</div>';
    if (job.status === 'interaction_required' && !job.error_code) body += `<p class="field-note">${escapeHTML(jobDiagnostic(job))}</p>`;
    if (job.status === 'committing') body += `<p class="field-note">${job.enable_credential ? '正在保存并启用 CPA 凭证，无需额外操作。' : '正在保存 CPA 凭证。'}此阶段不能取消。</p>`;
    if (activeStates.has(job.status) && job.phase) body += `<p class="field-note">${escapeHTML(phaseLabel(job.phase))}</p>`;
    if (cancellableStates.has(job.status)) body += '<button type="button" class="subtle small" data-login-action="cancel">取消登录任务</button>';
    body += `<small class="login-task-id">任务 ${escapeHTML(id)}</small>`;
    $('login-job-detail').innerHTML = heading + body;
    $('login-job-detail').querySelectorAll('[data-login-action]').forEach(button => { button.disabled = state.busy.has('action'); });
  }
  function selectJob(id) {
    state.selected = id; state.job = null; state.candidate = null; state.detailSignature = ''; state.detailRead++;
    state.revealJob = true;
    $('login-job-detail').innerHTML = '<p class="empty-line" role="status">正在读取任务…</p>';
  }
  async function loadSelected() {
    if (!state.selected) return;
    const epoch = state.epoch;
    const sequence = ++state.detailRead;
    const selected = state.selected;
    const result = await get(`/detail?id=${encodeURIComponent(selected)}`);
    if (!current(epoch) || state.selected !== selected || sequence !== state.detailRead) return;
    const job = result.job || result;
    state.job = job;
    if (job.authorization_url) state.urls.set(selected, safeAuthorizationURL(job.authorization_url));
    if (job.status === 'awaiting_commit') {
      const candidate = await get(`/candidate?id=${encodeURIComponent(job.candidate_id || selected)}`);
      if (!current(epoch) || state.selected !== selected || sequence !== state.detailRead) return;
      state.candidate = candidate.candidate || candidate;
    } else state.candidate = null;
    renderDetail();
  }
  async function reload() {
    if (!state.open || !connected()) return;
    const epoch = state.epoch;
    const sequence = ++state.jobsRead;
    clearTimeout(state.timer);
    const result = await get('/jobs?limit=200');
    if (!current(epoch) || sequence !== state.jobsRead) return;
    const previous = new Map(state.jobs.map(job => [idOf(job), job.status]));
    state.jobs = Array.isArray(result.items) ? result.items : [];
    const completed = state.jobs.filter(job => job.auto_commit && job.status === 'committed' && previous.has(idOf(job)) && previous.get(idOf(job)) !== 'committed');
    if (!state.selected && state.jobs.length) state.selected = idOf(state.jobs[0]);
    renderJobs();
    await Promise.all([loadSelected(), loadAccounts()]);
    await loadAccountLogs();
    if (!current(epoch) || sequence !== state.jobsRead) return;
    const securityCompleted = state.jobs.filter(job => isSecurityJob(job) && job.status === 'succeeded' && previous.has(idOf(job)) && previous.get(idOf(job)) !== 'succeeded');
    if (securityCompleted.length) feedback(`${securityCompleted.length === 1 ? securityCompleted[0].email || '账号' : `${securityCompleted.length} 个账号`}安全操作完成。${securityCompleted.length === 1 ? securityResult(securityCompleted[0]) : ''}`);
    if (completed.length) {
      const enabled = completed.every(job => job.credential_enabled);
      feedback(`${completed.length === 1 ? completed[0].email || '账号' : `${completed.length} 个账号`}${completed.every(isSecurityJob) ? '安全操作与重新授权完成' : '登录完成'}，${enabled ? '凭证已保存到 CPA 并启用' : '凭证已保存到 CPA'}。`);
      try { await onCommitted?.(); } catch { /* The account result stays visible if other workspace data is unavailable. */ }
    }
    schedule();
  }
  function schedule() {
    clearTimeout(state.timer);
    if (!state.open || !connected()) return;
    const epoch = state.epoch;
    state.timer = setTimeout(async () => {
      try { await reload(); }
      catch (error) { if (current(epoch) && !isDiscarded(error)) { feedback('任务状态暂未刷新。请检查连接，勿重复提交登录。', true); schedule(); } }
    }, state.jobs.some(job => activeStates.has(job.status)) ? 2000 : 6000);
  }
  function showDisconnected() {
    $('login-disconnected').classList.remove('hidden');
    $('login-workbench').classList.add('hidden');
  }
  async function open({ focus = true } = {}) {
    if (state.open) return;
    if (!connected()) {
      state.epoch++;
      clearTimeout(state.timer);
      showDisconnected();
      return;
    }
    state.open = true;
    const epoch = ++state.epoch;
    loading = true;
    state.capabilities = null;
    clearTimeout(state.timer);
    $('login-disconnected').classList.add('hidden');
    $('login-workbench').classList.remove('hidden');
    if (focus) $('login-title').focus({ preventScroll: true });
    feedback();
    updateControls();
    try {
      const [capabilities, targets] = await Promise.all([get('/capabilities'), get('/targets')]);
      if (!current(epoch)) return;
      state.capabilities = capabilities;
      state.targets = targets.items || [];
      await Promise.all([reload(), loadStorageStatus()]);
    } catch (error) {
      if (!current(epoch)) return;
      state.capabilities = { available: false, reason: 'capability_unavailable' };
      updateControls(); fail(error);
    } finally {
      if (current(epoch)) { loading = false; updateControls(); }
    }
  }
  function close({ reset = false, focus = true } = {}) {
    if (!state.open && !reset) return;
    state.open = false;
    loading = false;
    state.epoch++;
    clearTimeout(state.timer);
    clearSecrets();
    state.urls.clear();
    state.accountStarting = '';
    resetAccountLogs();
    state.candidate = null;
    state.job = null;
    state.detailSignature = '';
    state.jobSignature = ''; state.revealJob = false;
    closeAccountEditors();
    $('login-workbench').classList.add('hidden');
    $('login-disconnected').classList.toggle('hidden', connected());
    $('login-job-detail').innerHTML = '<p class="empty-line">选择任务查看进度和候选凭证。</p>';
    if (reset) {
      state.capabilities = null; state.targets = []; state.jobs = []; state.selected = ''; state.vault = null;
      state.accounts = []; state.accountSettings = null; state.accountsAvailable = false; state.accountSignature = ''; state.settingsDirty = false; state.settingsDraftRevision = null; state.accountQuery = ''; state.accountFilter = 'all'; state.accountPage = 0; $('account-search').value = '';
      $('account-list').replaceChildren(); $('account-count').textContent = ''; $('account-settings-form').reset(); $('account-settings-note').textContent = ''; renderAccountSettings(); renderAccounts();
      renderJobs(); feedback();
    }
    if (focus) document.querySelector('[data-page="login"]').focus({ preventScroll: true });
  }
  async function action(name, button) {
    if (state.busy.has('action') || !state.job) return;
    const epoch = state.epoch;
    const job = state.job;
    let body = { job_id: idOf(job), revision: job.revision };
    if (name === 'callback') {
      const callback = $('login-callback')?.value.trim();
      if (!callback) { feedback('请粘贴完整回调地址。', true); return; }
      body = { job_id: idOf(job), callback_url: callback };
      $('login-callback').value = '';
    }
    if (name === 'commit') {
      if (!state.candidate) return;
      body = { job_id: idOf(job), candidate_id: state.candidate.id, revision: state.candidate.revision };
    }
    if (!['callback','commit','cancel','discard'].includes(name)) return;
    state.busy.add('action'); button.disabled = true; feedback();
    try {
      const response = write(`/${name}`, body);
      if (body.callback_url) body.callback_url = '';
      await response;
      if (!current(epoch)) return;
      state.detailSignature = '';
      await reload();
      if (name === 'commit') await onCommitted?.();
    } catch (error) { if (current(epoch)) { fail(error); try { await reload(); } catch {} } }
    finally { if (body.callback_url) body.callback_url = ''; state.busy.delete('action'); if (current(epoch)) renderDetail(true); }
  }
  $('account-actions-menu').addEventListener('toggle', event => {
    if (event.newState === 'closed') closeAccountMenu();
  });
  $('account-actions-menu').addEventListener('keydown', event => {
    const buttons = [...$('account-actions-menu').querySelectorAll('button:not(:disabled)')];
    if (event.key === 'Escape') { event.preventDefault(); closeAccountMenu(true); }
    if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key) && buttons.length) {
      event.preventDefault();
      const index = buttons.indexOf(document.activeElement);
      const next = event.key === 'Home' ? 0 : event.key === 'End' ? buttons.length - 1 : (index + (event.key === 'ArrowDown' ? 1 : -1) + buttons.length) % buttons.length;
      buttons[next].focus();
    }
  });
  addEventListener('resize', () => closeAccountMenu());
  $('account-table-wrap').addEventListener('scroll', () => closeAccountMenu(), { passive: true });
  for (const [id, action] of [['account-reset-totp', 'reset_totp'], ['account-logout-all', 'logout_all']]) $(id).addEventListener('click', () => {
    const account = state.accounts.find(item => item.id === menuAccountID);
    if (account && !$(id).disabled) showAccountSecurity(account, action);
  });
  $('account-menu-delete').addEventListener('click', async () => {
    const account = state.accounts.find(item => item.id === menuAccountID);
    if (!account || accountBusy() || accountLoginPending(account)) return;
    closeAccountMenu(); await showAccountEditor(account);
    if (state.accountEditing?.id === account.id && $('account-editor-dialog').open && !state.accountDetailFailed) {
      $('account-delete-confirm').hidden = false; $('account-delete-no').focus();
    }
  });
  for (const id of ['account-security-close', 'account-security-cancel']) $(id).addEventListener('click', () => {
    if (!state.busy.has('account')) closeAccountSecurity(true);
  });
  $('account-security-dialog').addEventListener('cancel', event => {
    event.preventDefault();
    if (!state.busy.has('account')) closeAccountSecurity(true);
  });
  $('account-security-dialog').addEventListener('close', () => {
    if (!$('account-security-dialog').open) closeAccountSecurity(true);
  });
  $('account-security-form').addEventListener('submit', event => {
    event.preventDefault();
    if (accountBusy() || !securityAccount || !$('account-security-dialog').open || $('account-security-submit').disabled) return;
    const account = securityAccount;
    const body = { id: account.id, revision: account.revision, action: securityAction, logout_all_devices: securityAction === 'reset_totp' && $('account-security-logout').checked, reauthorize: $('account-security-reauthorize').checked };
    securityFeedback();
    accountAction(async epoch => {
      let result;
      try { result = await write('/accounts/security', body); }
      catch (error) {
        if (current(epoch) && (error instanceof TypeError || ['idempotency_unknown', 'security_pending'].includes(error.code))) {
          securitySubmissionUncertain = true;
          securityFeedback('提交结果尚未确认。请关闭窗口并查看账号任务，勿再次提交。');
        }
        throw error;
      }
      if (!current(epoch)) return;
      const job = result.job || result;
      state.jobs = [job, ...state.jobs.filter(item => idOf(item) !== idOf(job))];
      const saved = state.accounts.find(item => item.id === account.id);
      if (saved) Object.assign(saved, { last_job_id: idOf(job), last_job_status: job.status || 'queued', last_job_action: job.action, last_job_security_status: job.security_status });
      closeAccountSecurity(true);
      renderAccounts();
      feedback(`已开始${operationLabel(job)}，进度与结果会在账号行更新。`);
      await reload();
    });
  });
  $('account-security-complete').addEventListener('click', () => {
    const account = state.accountEditing;
    if (!account || accountBusy() || !securityRecoveryCompletable || !securityRecoveryJob) return;
    const body = { id: account.id, revision: account.revision, job_id: securityRecoveryJob };
    accountAction(async epoch => {
      await write('/accounts/security/complete', body);
      if (!current(epoch)) return;
      closeAccountEditor(true); await reload();
      if (current(epoch)) feedback('新 2FA 已完成本地保存。本次未执行退出设备、重新授权或上传 CPA。');
    });
  });
  document.querySelectorAll('[data-login-view]').forEach(button => button.addEventListener('click', () => setView(button.dataset.loginView)));
  $('account-active-jobs-open').addEventListener('click', () => setView('tasks'));
  for (const id of ['account-import-open', 'account-import-empty']) $(id).addEventListener('click', showAccountImport);
  for (const id of ['account-import-close', 'account-import-cancel']) $(id).addEventListener('click', () => {
    if (!state.busy.has('account')) closeAccountImport(true);
  });
  $('account-import-dialog').addEventListener('cancel', event => {
    event.preventDefault();
    if (!state.busy.has('account')) closeAccountImport(true);
  });
  $('account-import-dialog').addEventListener('close', () => {
    if (!$('account-import-dialog').open) closeAccountImport(true);
  });
  $('account-import-text').addEventListener('input', () => { importFeedback(); updateImportCount(); });
  for (const id of ['account-editor-close', 'account-editor-cancel']) $(id).addEventListener('click', () => {
    if (!state.busy.has('account')) closeAccountEditor(true);
  });
  $('account-editor-dialog').addEventListener('cancel', event => {
    event.preventDefault();
    if (!state.busy.has('account')) closeAccountEditor(true);
  });
  $('account-editor-dialog').addEventListener('close', () => {
    if (!$('account-editor-dialog').open) closeAccountEditor(true);
  });
  $('account-editor-retry').addEventListener('click', () => {
    const account = state.accounts.find(item => item.id === state.accountEditing?.id) || state.accountEditing;
    if (account) showAccountEditor(account);
  });
  $('account-settings-open').addEventListener('click', () => { setView('settings'); $('account-settings').open = true; $('account-default-proxy').focus(); });
  $('account-search').addEventListener('input', event => { state.accountQuery = event.target.value; state.accountPage = 0; renderAccounts(); });
  document.querySelectorAll('[data-account-filter]').forEach(button => button.addEventListener('click', () => { state.accountFilter = button.dataset.accountFilter; state.accountPage = 0; renderAccounts(); }));
  $('account-clear-filter').addEventListener('click', () => { state.accountQuery = ''; state.accountFilter = 'all'; state.accountPage = 0; $('account-search').value = ''; renderAccounts(); $('account-search').focus(); });
  $('account-page-prev').addEventListener('click', () => { state.accountPage = Math.max(0, state.accountPage - 1); renderAccounts(); });
  $('account-page-next').addEventListener('click', () => { state.accountPage++; renderAccounts(); });
  $('account-import-form').addEventListener('submit', event => {
    event.preventDefault();
    if (accountBusy() || !$('account-import-dialog').open || $('account-import-submit').disabled) return;
    let entries;
    try { entries = parseAccountImport($('account-import-text').value); }
    catch (error) { importFeedback(error.message); $('account-import-error').focus(); return; }
    importFeedback();
    const opener = importOpener;
    accountAction(async epoch => {
      const proxy = $('account-import-proxy').value.trim();
      const response = write('/accounts/import', { entries, ...(proxy ? { proxy_url: proxy } : {}) });
      $('account-import-text').value = '';
      $('account-import-proxy').value = '';
      updateImportCount();
      entries.forEach(entry => { entry.email = ''; entry.password = ''; entry.totp_secret = ''; });
      await response;
      if (!current(epoch)) return;
      closeAccountEditors();
      await loadAccounts();
      if (current(epoch)) feedback('账号已保存在本机。点击“2FA 登录”，成功后自动保存并启用 CPA 凭证。');
    }).finally(() => {
      entries.forEach(entry => { entry.email = ''; entry.password = ''; entry.totp_secret = ''; });
      if (state.open && connected() && state.view === 'accounts' && !$('account-import-dialog').open && opener?.isConnected && !opener.disabled) opener.focus({ preventScroll: true });
    });
  });
  $('account-inspect').addEventListener('click', () => accountAction(async epoch => {
    await write('/accounts/inspect', {});
    if (!current(epoch)) return;
    await loadAccounts();
    if (current(epoch)) feedback('已读取 CPA 记录的账号状态，本次巡检未发起登录。');
  }));
  $('account-settings-form').addEventListener('input', event => {
    if (!state.settingsDirty) state.settingsDraftRevision = state.accountSettings?.revision ?? null;
    state.settingsDirty = true;
    if (event.target.id === 'account-inspection-enabled' && !event.target.checked) $('account-auto-enabled').checked = false;
    updateAccountControls();
    $('account-settings-note').textContent = '有未保存更改';
  });
  $('account-settings-form').addEventListener('submit', event => {
    event.preventDefault();
    if (!$('account-settings-form').reportValidity() || !state.accountSettings) return;
    const body = { revision: state.settingsDraftRevision ?? state.accountSettings.revision, inspection_enabled: $('account-inspection-enabled').checked, auto_relogin_enabled: $('account-auto-enabled').checked, interval_minutes: Number($('account-interval').value), login_proxy_url: $('account-default-proxy').value.trim() };
    accountAction(async epoch => {
      try { await write('/accounts/settings', body, 'PUT'); }
      catch (error) {
        if (!current(epoch)) return;
        if (!['conflict', 'revision_conflict'].includes(error.code)) throw error;
        state.settingsDirty = false; state.settingsDraftRevision = null;
        await loadAccounts();
        if (current(epoch)) $('account-settings-note').textContent = state.accountsAvailable ? '设置已由其他操作更新，已载入最新值，请核对后再修改。' : '设置已由其他操作更新，等待重新读取后再修改。';
        return;
      }
      if (!current(epoch)) return;
      state.settingsDirty = false; state.settingsDraftRevision = null;
      await loadAccounts();
      if (current(epoch)) $('account-settings-note').textContent = '设置已保存。新登录使用保存的代理，已启用的巡检由宿主持续运行。';
    });
  });
  $('account-replace-secrets').addEventListener('change', () => {
    const replace = $('account-replace-secrets').checked;
    $('account-secret-editor').querySelectorAll('input').forEach(input => { input.required = replace; });
    updateAccountControls();
  });
  $('account-editor').addEventListener('submit', event => {
    event.preventDefault();
    if (accountBusy() || state.accountEditing?.security_pending || state.accountDetailLoading || state.accountDetailFailed || !$('account-editor-dialog').open || !$('account-editor').reportValidity() || !state.accountEditing) return;
    const account = state.accountEditing;
    const editorEpoch = state.epoch;
    const body = { id: account.id, revision: account.revision, label: $('account-label').value.trim(), auto_relogin: $('account-auto-relogin').checked, proxy_url: $('account-proxy-url').value.trim() };
    if ($('account-replace-secrets').checked) body.credentials = { email: account.email, password: $('account-password').value, totp_secret: $('account-totp').value.trim() };
    accountAction(async epoch => {
      const response = write('/accounts/entry', body, 'PUT');
      clearSecrets();
      if (body.credentials) Object.keys(body.credentials).forEach(name => { body.credentials[name] = ''; });
      try { await response; } catch (error) {
        if (current(epoch) && state.accountEditing?.id === account.id) {
          $('account-email').value = account.email || '';
          $('account-editor-note').textContent = '未能保存。关闭后重新打开可载入最新资料；更新登录资料需重新填写密码与 2FA。';
        }
        throw error;
      }
      if (!current(epoch)) return;
      clearSecrets(); closeAccountEditors(); await loadAccounts();
      if (current(epoch)) feedback('账号设置已保存。');
    }).finally(() => {
      if (body.credentials) Object.keys(body.credentials).forEach(name => { body.credentials[name] = ''; });
      if (current(editorEpoch) && state.view === 'accounts' && !$('account-editor-dialog').open) focusAccount(account.id);
    });
  });
  $('account-delete-open').addEventListener('click', () => { $('account-delete-confirm').hidden = false; $('account-delete-no').focus(); });
  $('account-delete-no').addEventListener('click', () => { $('account-delete-confirm').hidden = true; $('account-delete-open').focus(); });
  $('account-delete-yes').addEventListener('click', () => {
    const account = state.accountEditing;
    if (!account || accountBusy() || accountLoginPending(account)) return;
    accountAction(async epoch => {
      await write('/accounts/entry', { id: account.id, revision: account.revision }, 'DELETE');
      if (!current(epoch)) return;
      clearSecrets(); closeAccountEditors(); await loadAccounts();
      if (current(epoch)) feedback('已删除保存的账号及登录资料。CPA 凭证仍保留。');
    });
  });
  $('account-list').addEventListener('click', async event => {
    const button = event.target.closest('[data-account-action]');
    if (!button || button.disabled) return;
    const account = state.accounts.find(item => item.id === button.dataset.accountId);
    if (!account) return;
    if (button.dataset.accountAction === 'more') { showAccountMenu(account, button); return; }
    if (button.dataset.accountAction === 'logs') { toggleAccountLogs(account); return; }
    if (button.dataset.accountAction === 'edit' || button.dataset.accountAction === 'delete') {
      await showAccountEditor(account);
      if (button.dataset.accountAction === 'delete' && state.accountEditing?.id === account.id && $('account-editor-dialog').open && !state.accountDetailFailed) {
        $('account-delete-confirm').hidden = false; $('account-delete-no').focus();
      }
      return;
    }
    if (button.dataset.accountAction === 'inspect') {
      await accountAction(async epoch => {
        await write('/accounts/inspect', { id: account.id });
        if (!current(epoch)) return;
        await loadAccounts();
        if (current(epoch)) feedback('已读取此账号的 CPA 认证状态。');
      });
      return;
    }
    if (button.dataset.accountAction === 'task') {
      const epoch = state.epoch;
      selectJob(account.last_job_id);
      setView('tasks'); renderJobs();
      try { await loadSelected(); } catch (error) { if (current(epoch)) fail(error); }
      return;
    }
    if (button.dataset.accountAction === 'start') await startAccount(account);
  });
  $('account-login-start').addEventListener('click', () => {
    const account = state.accounts.find(item => item.id === state.accountEditing?.id) || state.accountEditing;
    if (account && $('account-editor-dialog').open && !state.accountDetailLoading && !state.accountDetailFailed) startAccount(account, $('account-login-target').value);
  });
  $('login-connect').addEventListener('click', () => {
    document.getElementById('connection-dialog').showModal();
  });
  $('login-jobs').addEventListener('click', async event => {
    const button = event.target.closest('[data-login-job]');
    if (!button) return;
    const epoch = state.epoch;
    clearSecrets(); selectJob(button.dataset.loginJob);
    renderJobs(); try { await loadSelected(); } catch (error) { if (current(epoch)) fail(error); }
  });
  $('login-job-detail').addEventListener('click', event => { const button = event.target.closest('[data-login-action]'); if (button && !button.disabled) action(button.dataset.loginAction, button); });
  $('login-refresh').addEventListener('click', async () => { const epoch = state.epoch; try { feedback(); await reload(); } catch (error) { if (current(epoch)) fail(error); } });
  $('login-export').addEventListener('click', () => {
    const report = state.jobs.map(job => ({ id: idOf(job), method: ['browser', 'password_totp'].includes(job.method) ? job.method : 'unknown', intent: ['import', 'reauthorize'].includes(job.intent) ? job.intent : 'unknown', email: job.email, target_auth_id: job.target_auth_id, account_id: job.account_id, automatic: !!job.automatic, auto_commit: !!job.auto_commit, credential_enabled: !!job.credential_enabled, action: isSecurityJob(job) ? job.action : undefined, security_status: job.security_status, totp_reset: !!job.totp_reset, devices_logged_out: !!job.devices_logged_out, status: labels[job.status] ? job.status : 'unknown', phase: phaseLabel(job.phase), error_code: reasons[job.error_code] ? job.error_code : job.error_code ? 'unknown' : undefined, created_at: job.created_at }));
    const url = URL.createObjectURL(new Blob([JSON.stringify({ jobs: report }, null, 2)], { type: 'application/json' }));
    const anchor = document.createElement('a'); anchor.href = url; anchor.download = 'openai-login-report.json'; anchor.click(); URL.revokeObjectURL(url);
  });
  return { open, close, reset: () => close({ reset: true, focus: false }) };
}

function phaseLabel(phase) {
  return ({ security_login: '登录账号安全会话', security_check: '核对账号与 2FA 状态', totp_disable: '移除旧 2FA', totp_enroll: '登记新 2FA', totp_activate: '激活新 2FA', totp_saved: '新 2FA 已保存', logout_all: '退出所有设备', reauthorization: '重新授权 CPA', security_complete: '安全操作完成', queued: '等待执行', starting: '准备登录环境', runtime_start: '启动 Go 登录引擎', browser_start: '启动登录环境', session: '建立网页登录会话', authorization: '正在进行登录授权', email: '填写账号邮箱', password: '验证账号密码', totp: '验证 2FA', mfa: '验证 TOTP', oauth: '获取 Codex 授权', callback: '接收授权回调', browser_verification: '处理网站验证', consent: '确认授权', workspace: '核对账号工作区', authorizing: '获取官方授权', token_exchange: '兑换并核验授权', exchanging: '兑换授权', validating: '核验候选身份', review: '等待核对候选凭证', saving: '保存候选凭证' })[phase] || '正在处理登录任务';
}
function diagnosticStage(stage) {
  if (['security_login', 'security_check', 'totp_disable', 'totp_enroll', 'totp_activate', 'totp_saved', 'logout_all', 'reauthorization', 'security_complete'].includes(stage)) return phaseLabel(stage);
  return ({ runtime_start: '启动 Go 登录引擎', browser_start: '启动登录环境', session: '建立网页登录会话', authorization: '打开授权页', email: '填写邮箱', password: '验证密码', totp: '验证 2FA', oauth: '获取 Codex 授权', browser_verification: '处理网站验证', consent: '确认授权', workspace: '选择工作区', callback: '接收回调' })[stage] || '登录流程';
}
function jobDiagnostic(job) {
  const reason = ({ human_verification: '需要完成人机验证', browser_verification_timeout: '网站自动检查超时', browser_verification_required: '网站验证尚未完成', email_verification: '需要邮箱验证码', phone_verification: '需要手机验证', unsupported_request: '当前登录运行时尚未适配此请求', unsupported_navigation: '遇到暂不支持的页面跳转', submit_unavailable: '未找到继续按钮', totp_method_unconfirmed: '无法确认当前验证码方式', workspace_selection: '需要选择工作区', account_selection: '需要选择账号', unexpected_dialog: '出现额外对话框', unexpected_download: '出现非预期下载', unsupported_page: '未识别当前登录页面', upstream_rejected: '官方页面拒绝验证', proxy_connection_failed: '代理连接失败', session_unconfirmed: '未确认网页登录会话', authentication_not_advanced: '验证响应未确认流程前进', totp_factor_missing: '未找到可用的 TOTP 验证方式', sentinel_unavailable: '官方登录安全组件未就绪' })[job.error_reason];
  if (reason) return `${job.error_stage ? `${diagnosticStage(job.error_stage)}：` : ''}${reason}`;
  if (job.status === 'interaction_required' || job.error_code === 'interaction_required') return '旧记录未保存停止步骤，请查看任务。';
  if (job.error_code) return messageFor(job.error_code);
  return '';
}
function diagnosticAdvice(job) {
  if (job.status === 'security_uncertain') return '请查看账号资料保留的安全操作信息与新密钥。未确认的修改不会自动重试，旧 2FA 可能已失效。';
  if (isSecurityJob(job) && job.totp_reset) return '新 2FA 已保存，旧密钥不会恢复。请仅处理失败的后续步骤，不要为了重新授权再次重置 2FA。';
  if (isSecurityJob(job) && job.devices_logged_out) return '退出所有设备已确认完成。请仅处理失败的后续授权，无需再次退出。';
  if (job.error_reason === 'browser_verification_timeout') return '官方页面的自动检查未在等待时间内完成，尚未确认需要人工验证。请保留任务日志并检查代理连接，本次任务不会自动重试。';
  if (job.error_reason === 'browser_verification_required') return '官方接口返回了网站验证页面，当前认证步骤未完成。此响应未确认密码或 2FA 错误，也未确认需要人工验证。';
  if (job.error_reason === 'session_unconfirmed') return '尚未确认网页会话登录成功，未继续获取 Codex 授权。请核对官方账号要求和代理连接后再手动重试。';
  if (job.error_reason === 'authentication_not_advanced') return '已收到验证响应，但没有确认下一步认证状态，未判定验证成功。请保留任务的停止步骤和错误原因以便排查。';
  if (job.error_reason === 'totp_factor_missing') return '官方返回的认证会话中没有可用的 TOTP 因子。请核对账号的验证方式；本次未提交 2FA 验证码。';
  if (job.error_reason === 'sentinel_unavailable') return '官方登录安全组件未能正常加载或返回所需信息。请检查代理能否访问官方认证资源，再手动重试。';
  if (job.error_reason === 'unsupported_request') return '需适配登录运行时后再重试。当前记录未保存被拦截的请求路径，无法确定具体端点；这不代表密码或 2FA 错误。';
  if (job.error_reason === 'proxy_connection_failed') return '检查代理地址、端口和认证资料后，再手动登录。可在“代理与巡检”中设置默认代理。';
  if (['human_verification', 'email_verification', 'phone_verification', 'workspace_selection', 'account_selection'].includes(job.error_reason)) return '此步骤需要人工处理，当前自动登录无法继续。请核对官方账号要求后再决定是否重新登录，本次任务不会自动重试。';
  if (['unsupported_navigation', 'submit_unavailable', 'totp_method_unconfirmed', 'unsupported_page'].includes(job.error_reason)) return '当前登录页面与自动流程不匹配，需要适配登录运行时。请保留此任务的错误代码与停止步骤以便排查。';
  if (job.status === 'interaction_required' && !job.error_reason) return '此历史记录不能判断是页面兼容问题还是额外验证。新版任务会记录停止步骤和原因，核对代理及资料后可手动重试。';
  if (['invalid_proxy', 'proxy_auth_unsupported'].includes(job.error_code)) return '在账号资料或“代理与巡检”中修改代理后再手动登录。';
  return '本次任务已停止。核对账号资料、网络及 CPA 状态后，再决定是否手动重试。';
}
function reasonLabel(reason) {
  return ({ not_inference_verified: '此授权尚未经过模型测试', quality_paused: '质量暂停继续生效', operator_disabled: '人工禁用继续生效', manual_disabled: '人工禁用继续生效', native_cooldown: '原生冷却继续生效', rules_stopped: '相关检测规则保持停止', new_credential_disabled: '新凭证默认人工禁用' })[reason] || '其他凭证限制继续生效';
}

export function parseAccountImport(text) {
  const lines = text.split(/\r?\n/).filter(line => line.trim());
  if (!lines.length || lines.length > 100) throw new Error('请填写 1–100 行账号资料。');
  const seen = new Set();
  return lines.map((line, index) => {
    const first = line.indexOf('----');
    const last = line.lastIndexOf('----');
    if (first < 1 || first === last) throw new Error(`第 ${index + 1} 行格式不完整，请使用“账号----密码----2fa”。`);
    const email = line.slice(0, first).trim().toLowerCase();
    const password = line.slice(first + 4, last);
    const totp_secret = line.slice(last + 4).trim();
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email) || email.length > 254 || !password || password.length > 4096 || !totp_secret || totp_secret.length > 1024) throw new Error(`第 ${index + 1} 行的邮箱、密码或 2FA 密钥无效，请核对后重试。`);
    if (seen.has(email)) throw new Error(`第 ${index + 1} 行与前面的账号重复，请移除重复项。`);
    seen.add(email);
    return { email, password, totp_secret };
  });
}
function healthLabel(status) {
  return ({ not_linked: '尚未绑定', unbound: '尚未绑定', not_checked: '尚未巡检', unknown: '状态未知', healthy: '未记录鉴权异常', available: '未记录鉴权异常', ok: '未记录鉴权异常', needs_reauth: '需要重新授权', requires_sign_in: '需要重新登录', auth_failed: '鉴权失效', missing: 'CPA 凭证缺失', identity_changed: '凭证身份已变化', disabled: '人工禁用', manual_disabled: '人工禁用', paused: '质量暂停', quality_paused: '质量暂停', cooldown: '冷却中', native_cooldown: '冷却中', unsupported: '不支持巡检', unavailable: '暂不可用', refresh_pending: '等待原生刷新', blocked: '暂不可自动处理', pending: '待核对', no_auth_failure: '未记录鉴权异常' })[status] || '待核对';
}
function healthReason(reason) {
  return ({ login_required: '2FA 登录成功后自动绑定', awaiting_inspection: '等待首次巡检', no_auth_failure_observed: '未记录认证异常', credential_deleted: '不会自动重建凭证', unbound: '2FA 登录成功后自动绑定', not_found: '不会自动重建凭证', missing: '不会自动重建凭证', requires_sign_in: 'CPA 标记需要重新登录', unauthorized: 'CPA 记录鉴权失败', invalid_grant: '授权已失效', token_expired: '等待 CPA 原生刷新', native_refresh_pending: '等待 CPA 原生刷新完成', native_unavailable: 'CPA 暂未标记为可用；不会据此自动重登', credential_login_pending: '等待上次授权提交完成', refresh_outcome_unknown: '刷新结果未确认；需人工重新授权', siwc_sign_in_required: 'SIWC 授权需人工重新登录', quality_identity_unavailable: '持久身份状态尚未就绪', login_source_conflict: '凭证来源存在冲突，需人工核对', login_source_unavailable: '凭证来源暂不可读取', login_target_unavailable: 'CPA 凭证状态暂不可读取', quality_paused: '跳过自动重新授权', manual_disabled: '跳过自动重新授权', operator_disabled: '跳过自动重新授权', native_cooldown: '等待冷却结束', cooldown: '等待冷却结束', identity_changed: '需人工核对绑定身份', unsupported_login_target: '当前来源不支持更新', auth_failure: 'CPA 记录鉴权失败', no_auth_failure: '未发现需要重登的记录', login_commit_pending: '等待上次提交完成', pending_commit: '等待上次提交完成', interaction_required: '上次自动登录未完成，请查看任务原因', credentials_rejected: '请更新密码后手动重试', totp_rejected: '请更新 2FA 资料后手动重试', backoff: '等待下次允许重试', rate_limited: '达到登录频率限制', retry_exhausted: '已停止自动重试', plugin_unavailable: '登录服务暂不可用' })[reason] || reasons[reason] || '请结合 CPA 凭证状态核对';
}
function automaticError(code) {
  if (['runtime_timeout', 'runtime_failed', 'exchange_failed', 'rate_limited', 'network_error', 'timeout'].includes(code)) return '本次自动授权未完成。宿主将按退避和次数上限处理，请在账号列表查看后续状态。';
  return messageFor(code);
}

function needsAttention(account) {
  return !!account.attention_required || !!account.security_pending || ['needs_reauth', 'requires_sign_in', 'auth_failed', 'missing', 'identity_changed'].includes(account.health_status) || ['commit_unknown', 'saved_not_adopted', 'security_uncertain'].includes(account.last_job_status);
}
function compactDate(value) {
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? '时间未知' : new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }).format(parsed);
}

function accountIcon(name) {
  const paths = { more: 'M3 10h.01M10 10h.01M17 10h.01', login: 'M11 3h5v14h-5M3 10h10M9 6l4 4-4 4', user: 'M13 6a3 3 0 1 1-6 0 3 3 0 0 1 6 0M4 17v-2c0-5 12-5 12 0v2', delete: 'M3 5h14M7 5V3h6v2M5 5l1 12h8l1-12M8 8v6M12 8v6', task: 'M6 3h8v3H6zM6 5H4v13h12V5h-2M7 10h6M7 14h6', logs: 'M4 3h12v14H4zM7 7h6M7 10h6M7 13h4', refresh: 'M16 7a6.5 6.5 0 1 0 .4 5M16 3v4h-4', copy: 'M7 7h10v10H7zM13 7V3H3v10h4', chevron: 'm7 5 5 5-5 5' };
  return `<svg class="login-control-icon" viewBox="0 0 20 20" aria-hidden="true"><path d="${paths[name] || paths.logs}"/></svg>`;
}
function logTime(value) {
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? '时间未知' : parsed.toLocaleTimeString('zh-CN', { hour12: false });
}
function logEntrySummary(entry, job) {
  const title = isSecurityJob(job) ? jobLabel({ ...job, ...entry }) : entry.status === 'interaction_required' ? '自动登录已停止' : labels[entry.status] || '状态已更新';
  const phase = entry.error_stage ? diagnosticStage(entry.error_stage) : entry.phase ? phaseLabel(entry.phase) : '';
  const reason = entry.error_reason ? logReasonLabel(entry.error_reason) : entry.error_code ? messageFor(entry.error_code) : '';
  return { title, detail: [phase, reason].filter(Boolean).join('：') };
}
function logReasonLabel(reason) {
  return ({
    human_verification: '需要完成人机验证', browser_verification_timeout: '网站自动检查超时', browser_verification_required: '网站验证尚未完成', email_verification: '需要邮箱验证码', phone_verification: '需要手机验证',
    unsupported_request: '授权页发出了当前登录运行时尚未适配的请求', unsupported_navigation: '暂不支持当前页面跳转', submit_unavailable: '未找到继续按钮',
    totp_method_unconfirmed: '无法确认验证码方式', workspace_selection: '需要选择工作区', account_selection: '需要选择账号',
    unexpected_dialog: '出现额外对话框', unexpected_download: '出现非预期下载', unsupported_page: '未识别当前登录页面',
    upstream_rejected: '官方页面拒绝验证', proxy_connection_failed: '代理连接失败',
    session_unconfirmed: '未确认网页登录会话', authentication_not_advanced: '验证响应未确认流程前进',
    totp_factor_missing: '未找到可用的 TOTP 验证方式', sentinel_unavailable: '官方登录安全组件未就绪',
  })[reason] || reason;
}
