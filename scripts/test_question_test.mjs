import assert from 'node:assert/strict';
import { auditLabel, credentialBlock, questionTestSelection, reasonLabel, renderHistoryResult, renderRunEvidence, statusTag, verdictTag } from '../plugin/ui/view.js';

const question = { id: 'logic-candy', title: '糖果逻辑测试', kind: 'logic', answer: '21', prompt: 'Only this exact prompt owns the reference answer.', version: 'v1' };
assert.deepEqual(questionTestSelection(question, question.prompt), { question_id: 'logic-candy' });
for (const prompt of [`${question.prompt} `, question.prompt.replace('exact', 'changed'), 'A different question']) {
  assert.deepEqual(questionTestSelection(question, prompt), { question_id: 'custom', prompt });
}
assert.deepEqual(questionTestSelection(undefined, 'My question'), { question_id: 'custom', prompt: 'My question' });

const run = {
  source: 'manual_test', status: 'completed', action_status: 'not_applicable', created_at: '2026-11-20T13:00:00Z',
  config: { models: ['example-model'], samples: 1, question, judge: { mode: 'manual' } },
  samples: [{ model: 'example-model', index: 0, execution_status: 'completed', quality_verdict: 'unknown', reason: 'manual assessment required', output: '<svg onload="alert(1)"></svg>', usage_known: false }],
};
const simple = renderRunEvidence(run);
const recordedPolicy = { code: 'error_policy_disabled', reason: 'CPA 注册目录缺少所选模型', model: 'old-model' };
const external = { auth_id: 'fake-active', credential_handling: 'disabled', error_policy_block: recordedPolicy, runtime_status: 'active', effective_available: true, models: ['example-model'] };
const manual = { manual: true };
const config = { auth_id: external.auth_id, models: ['example-model'] };
assert.equal(credentialBlock(config, [external], manual), '');
assert.match(credentialBlock({ ...config, models: ['old-model'] }, [external], manual), /当前目录没有所选模型 old-model/);
assert.match(credentialBlock(config, [{ ...external, retained_pause: true }], manual), /质量暂停/);
assert.match(credentialBlock(config, [{ ...external, runtime_status: 'disabled' }], manual), /仍停用/);
assert.match(credentialBlock(config, [{ ...external, effective_available: false }], manual), /未确认/);
assert.equal(credentialBlock(config, [{ ...external, error_policy_block: null }], manual), '');
assert.match(credentialBlock({ control_auth_id: external.auth_id }, [external], manual), /对照凭证已有错误策略/);
assert.match(credentialBlock({ judge: { mode: 'llm', auth_id: external.auth_id } }, [external], manual), /判题凭证已有错误策略/);
assert.match(credentialBlock(config, [external]), /规则、对照和判题/);
const blockedHistory = { ...run, status: 'blocked', samples: [], error: 'blocked: credential already disabled by error policy' };
const blockedSummary = renderRunEvidence(blockedHistory).metaHTML;
assert.match(blockedSummary, /当前凭证状态尚未确认，无法开始智商测试。请刷新后重试。/);
assert.doesNotMatch(blockedSummary, /blocked|credential already disabled|错误策略|质量运维|处置/);
assert.equal(blockedHistory.error, 'blocked: credential already disabled by error policy');
for (const [answer_match, individual, overall] of [['correct', '答案正确', '正确'], ['incorrect', '答案错误', '错误'], ['unknown', '未判定', '未判定']]) {
  const one = renderRunEvidence({ ...run, samples: [{ ...run.samples[0], output: '29', answer_match }] });
  assert(one.samplesHTML.includes(`data-verdict="${answer_match}"`));
  assert(one.samplesHTML.includes('>' + individual + '<'));
  assert(one.conditionsHTML.includes('>' + overall + '<'));
  assert.doesNotMatch(one.metaHTML, />已完成<|>正确<|>错误<|>未判定</);
  assert.equal(one.samplesHTML.includes('<dt>标准答案</dt><dd>21</dd>'), answer_match === 'incorrect');
}
const wrongSample = { ...run.samples[0], output: '29', answer_match: 'incorrect' };
for (const sample of [{ ...wrongSample, review: 'unknown' }, { ...wrongSample, execution_status: 'failed' }]) {
  assert.doesNotMatch(renderRunEvidence({ ...run, samples: [sample] }).samplesHTML, /class="answer-reference"/);
}
const customConfig = { ...run.config, question: { ...question, kind: 'custom' } };
assert.doesNotMatch(renderRunEvidence({ ...run, config: customConfig, samples: [wrongSample] }).samplesHTML, /class="answer-reference"/);
assert.equal(simple.completed, 1);
assert.equal(simple.planned, 1);
assert.match(simple.metaHTML, /<h3[^>]*>糖果逻辑测试<\/h3>/);
assert.match(simple.metaHTML, />example-model</);
assert.match(simple.metaHTML, /<time datetime="2026-11-20T13:00:00Z">/);
assert.match(simple.samplesHTML, />模型回答</);
assert.doesNotMatch(simple.samplesHTML, /class="sample-number"/);
assert.doesNotMatch(renderRunEvidence({ ...run, created_at: undefined }).metaHTML, /<time/);
assert.doesNotMatch(simple.metaHTML, /无法判断|动作：/);
assert.doesNotMatch(simple.samplesHTML, /sample-state-label|需要人工评审|data-review-sample/);
assert.match(simple.samplesHTML, /&lt;svg onload=&quot;alert\(1\)&quot;&gt;/);
assert.doesNotMatch(simple.samplesHTML, /<svg onload/);
assert.match(simple.samplesHTML, /预览 HTML \/ SVG/);

const scheduled = renderRunEvidence({ ...run, source: 'scheduled_probe' });
assert.match(scheduled.metaHTML, /无法判断/);
assert.match(scheduled.samplesHTML, /sample-state-label/);
assert.match(scheduled.samplesHTML, /data-review-sample/);
const partial = renderRunEvidence({...run, source:'scheduled_probe', action_status:'insufficient_evidence',
  config:{...run.config, samples:2}, samples:[{...run.samples[0], comparable:false, quality_verdict:'correct'}]});
assert.match(partial.metaHTML, /正确 1/);
assert.match(partial.metaHTML, /1 \/ 2 样本已结束/);
assert.match(partial.metaHTML, /整轮判定依据不足，未处理凭证/);
assert.doesNotMatch(partial.samplesHTML, /证据不足/);

// Duplicate and out-of-plan records stay readable without completing a missing position.
const duplicate = { ...run, config: { ...run.config, samples: 2 }, samples: [
  { ...run.samples[0], output: 'duplicate A', answer_match: 'correct' },
  { ...run.samples[0], output: 'duplicate B', answer_match: 'correct' },
] };
assert.match(renderRunEvidence(duplicate).conditionsHTML, /部分已判定/);
assert.doesNotMatch(renderHistoryResult(duplicate), /class="history-score"/, 'Repeated model/sample positions cannot complete the history score');
const endedSample = { ...run.samples[0], answer_match: 'correct' };
for (const status of ['running', 'cancelled', 'failed']) {
  assert.doesNotMatch(renderHistoryResult({ ...run, status, samples: [endedSample] }), /class="history-score"/, status);
}
const anomalous = renderRunEvidence({ ...duplicate, samples: [...duplicate.samples,
  { ...run.samples[0], model: 'other-model', output: 'unplanned model' },
  { ...run.samples[0], index: -1, output: 'negative index' },
  { ...run.samples[0], index: 4, output: 'out of range' },
  { ...run.samples[0], index: undefined, output: 'unspecified index' },
] });
assert.equal(anomalous.completed, 1);
assert.equal(anomalous.planned, 2);
for (const answer of ['duplicate A', 'duplicate B', 'unplanned model', 'negative index', 'out of range', 'unspecified index']) {
  assert(anomalous.samplesHTML.includes('>' + answer + '</pre>'));
}
assert.match(anomalous.samplesHTML, /回答缺失/);
const trimmed = { ...run, source: 'scheduled_probe', pruned_samples: 1,
  config: { ...run.config, samples: 3 }, samples: [{ ...run.samples[0], index: 1 }],
  pruned_sample_slots: [{ model: 'example-model', index: 2 }],
};
const knownSlots = renderRunEvidence(trimmed);
assert.match(knownSlots.samplesHTML, /样本 1<\/span><\/h4><span class="tag warn">样本缺失/);
assert.match(knownSlots.samplesHTML, /样本 3<\/span><\/h4><span class="tag neutral">历史已裁剪/);
const partiallyKnown = renderRunEvidence({ ...trimmed, pruned_samples: 2 });
assert.match(partiallyKnown.samplesHTML, /样本 1<\/span><\/h4><span class="tag neutral">无留存回答/);
assert.match(partiallyKnown.samplesHTML, /样本 3<\/span><\/h4><span class="tag neutral">历史已裁剪/);

const failed = renderRunEvidence({ ...run, status: 'failed', error: 'network unavailable', samples: [{ ...run.samples[0], execution_status: 'failed', output: '', error: 'upstream disconnected' }] });
assert.match(failed.metaHTML, /network unavailable/);
assert.match(failed.samplesHTML, /upstream disconnected/);
assert.match(failed.samplesHTML, /执行失败/);
assert.match(failed.metaHTML, />执行失败</);
assert.doesNotMatch(failed.metaHTML, /错误 1/);
const mixed = renderRunEvidence({ ...run, samples: [
  { ...run.samples[0], answer_match: 'correct' },
  { ...run.samples[0], index: 1, answer_match: 'incorrect' },
  { ...run.samples[0], index: 2, review: 'unknown', answer_match: 'correct' },
  { ...run.samples[0], index: 3, quality_verdict: 'incorrect', answer_match: 'correct' },
  { ...run.samples[0], index: 4, review: 'correct', quality_verdict: 'incorrect' },
] });
assert.match(mixed.conditionsHTML, />部分正确</);
assert.doesNotMatch(mixed.metaHTML, /部分正确/);
assert.doesNotMatch(mixed.metaHTML, /(正确|错误|未判定)\s*\d/);
assert.match(mixed.samplesHTML, />答案正确</);
assert.match(mixed.samplesHTML, />答案错误</);
assert.match(mixed.samplesHTML, />回答 2</);
for (const reason of ['cancelled', 'context canceled']) assert.equal(reasonLabel(reason), '测试已取消');
assert.equal(reasonLabel('unmapped upstream error'), 'unmapped upstream error');
assert.equal(auditLabel('credential_external_enable'), '已识别 CPA 人工启用');
assert.equal(auditLabel('credential_external_change'), '已识别 CPA 当前凭证变更');
const cancelledSample = { ...run.samples[0], execution_status: 'cancelled', error: 'context canceled', output: '' };
const cancelled = renderRunEvidence({ ...run, status: 'cancelled', samples: [cancelledSample] });
assert.match(cancelled.samplesHTML, /测试已取消/);
assert.equal(cancelledSample.error, 'context canceled');
for (const status of ['completed', 'complete', 'success', 'succeeded', 'released']) {
  assert.doesNotMatch(statusTag(status), /tag good/, 'Execution completion and owned release must not imply correct quality or full availability');
}
assert.match(verdictTag('correct'), /tag good/, 'Only an explicitly correct verdict uses the quality success color');
console.log('Question test prompt binding, manual result presentation, escaping and rule-view compatibility passed.');
