import assert from 'node:assert/strict';
import { auditLabel, questionTestSelection, reasonLabel, renderManualVerdict, renderRunEvidence, statusTag, verdictTag } from '../plugin/ui/view.js';

const question = { id: 'logic-candy', prompt: 'Only this exact prompt owns the reference answer.', version: 'v1' };
assert.deepEqual(questionTestSelection(question, question.prompt), { question_id: 'logic-candy' });
for (const prompt of [`${question.prompt} `, question.prompt.replace('exact', 'changed'), 'A different question']) {
  assert.deepEqual(questionTestSelection(question, prompt), { question_id: 'custom', prompt });
}
assert.deepEqual(questionTestSelection(undefined, 'My question'), { question_id: 'custom', prompt: 'My question' });

const run = {
  source: 'manual_test', status: 'completed', action_status: 'not_applicable',
  config: { models: ['example-model'], samples: 1, question, judge: { mode: 'manual' } },
  samples: [{ model: 'example-model', index: 0, execution_status: 'completed', quality_verdict: 'unknown', reason: 'manual assessment required', output: '<svg onload="alert(1)"></svg>', usage_known: false }],
};
const simple = renderRunEvidence(run);
for (const [answer_match, text] of [['correct', '<span class="tag good">正确</span>'], ['incorrect', '<span class="tag warn">错误</span>'], ['unknown', '<span class="tag neutral">未判定</span>']]) {
  const one = { ...run, samples: [{ ...run.samples[0], answer_match }] };
  assert.equal(renderManualVerdict(one), text);
  assert(renderRunEvidence(one).metaHTML.includes(text));
}
assert.equal(simple.completed, 1);
assert.equal(simple.planned, 1);
assert.match(simple.metaHTML, />未判定</);
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
assert.match(renderManualVerdict(duplicate), /部分已判定/);
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
assert.match(mixed.metaHTML, /tag warn">部分正确</);
assert.doesNotMatch(mixed.metaHTML, /(正确|错误|未判定)\s*\d/);
assert.match(mixed.samplesHTML, /tag good">答案正确/);
assert.match(mixed.samplesHTML, /tag warn">答案错误/);
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
