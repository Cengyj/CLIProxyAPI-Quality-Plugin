import assert from 'node:assert/strict';
import { questionTestSelection, reasonLabel, renderRunEvidence, statusTag, verdictTag } from '../plugin/ui/view.js';

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
assert.equal(simple.completed, 1);
assert.equal(simple.planned, 1);
assert.match(simple.metaHTML, /不自动评分/);
assert.doesNotMatch(simple.metaHTML, /无法判断|动作：/);
assert.doesNotMatch(simple.samplesHTML, /sample-state-label|需要人工评审|data-review-sample/);
assert.match(simple.samplesHTML, /&lt;svg onload=&quot;alert\(1\)&quot;&gt;/);
assert.doesNotMatch(simple.samplesHTML, /<svg onload/);
assert.match(simple.samplesHTML, /预览 HTML \/ SVG/);

const scheduled = renderRunEvidence({ ...run, source: 'scheduled_probe' });
assert.match(scheduled.metaHTML, /无法判断/);
assert.match(scheduled.samplesHTML, /sample-state-label/);
assert.match(scheduled.samplesHTML, /data-review-sample/);

const failed = renderRunEvidence({ ...run, status: 'failed', error: 'network unavailable', samples: [{ ...run.samples[0], execution_status: 'failed', output: '', error: 'upstream disconnected' }] });
assert.match(failed.metaHTML, /network unavailable/);
assert.match(failed.samplesHTML, /upstream disconnected/);
assert.match(failed.samplesHTML, /执行失败/);
for (const reason of ['cancelled', 'context canceled']) assert.equal(reasonLabel(reason), '测试已取消');
assert.equal(reasonLabel('unmapped upstream error'), 'unmapped upstream error');
const cancelledSample = { ...run.samples[0], execution_status: 'cancelled', error: 'context canceled', output: '' };
const cancelled = renderRunEvidence({ ...run, status: 'cancelled', samples: [cancelledSample] });
assert.match(cancelled.samplesHTML, /测试已取消/);
assert.equal(cancelledSample.error, 'context canceled');
for (const status of ['completed', 'complete', 'success', 'succeeded', 'released']) {
  assert.doesNotMatch(statusTag(status), /tag good/, 'Execution completion and owned release must not imply correct quality or full availability');
}
assert.match(verdictTag('correct'), /tag good/, 'Only an explicitly correct verdict uses the quality success color');
console.log('Question test prompt binding, manual result presentation, escaping and rule-view compatibility passed.');
