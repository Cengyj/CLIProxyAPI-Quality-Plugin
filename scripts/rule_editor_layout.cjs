const assert = require('node:assert/strict');

// Measure the actual modal inside its iframe, including scroll and theme behavior.
module.exports = async function checkRuleLayout(editor) {
  const box = await editor.evaluate(root => {
    const rect = s => root.querySelector(s).getBoundingClientRect().toJSON();
    const style = getComputedStyle(root.querySelector('.model-deck'));
    return { modal:root.getBoundingClientRect().toJSON(), open:root.open, tag:root.tagName,
      form:rect('form'), body:rect('.rule-form-scroll'), header:rect('.rule-masthead'), footer:rect('.editor-footer'),
      identity:rect('.rule-identity'), instrument:rect('.rule-instrument'), selection:rect('.deck-selection'),
      composer:rect('.deck-edit'), entry:rect('.rule-model-entry'), shortcuts:rect('.deck-shortcuts'),
      caption:rect('.deck-heading'), modelTitle:rect('#rule-model-title'), count:rect('#rule-model-count'),
      sampling:rect('.sampling-strip'), effort:rect('.effort-scale'), quantity:rect('.rule-stepper'), judge:rect('[aria-labelledby="rule-question-title"]'),
      plan:rect('[aria-labelledby="rule-plan-title"]'), name:rect('#rule-name'), scope:rect('.rule-context'),
      inputSize:parseFloat(getComputedStyle(root.querySelector('#rule-name')).fontSize), theme:document.documentElement.dataset.theme,
      colorScheme:style.colorScheme, color:style.backgroundColor, width:innerWidth, height:innerHeight,
      outerScroll:root.scrollTop, outerExtra:root.scrollHeight-root.clientHeight, overflows:root.scrollWidth>root.clientWidth+1 || root.querySelector('.rule-form-scroll').scrollWidth>root.querySelector('.rule-form-scroll').clientWidth+1 };
  });
  assert.equal(box.tag,'DIALOG'); assert(box.open);
  assert.equal(box.outerScroll,0,'The modal itself must not retain a hidden scroll offset');
  assert(box.outerExtra<=2,'Accessible descriptions must stay inside the scrolling body');
  assert(!box.overflows, 'Modal must not overflow horizontally');
  assert(box.modal.x>=0 && box.modal.right<=box.width+1 && box.modal.y>=0 && box.modal.bottom<=box.height+1,'Modal stays inside iframe');
  assert(box.header.bottom<=box.body.y+1 && box.body.bottom<=box.footer.y+1,'Only the form body scrolls');
  assert(box.footer.bottom<=box.modal.bottom+1,'Footer stays visible');
  for(const key of ['instrument','judge','plan']) assert(Math.abs(box[key].width-box.identity.width)<1,'Content uses a consistent width');
  const below=(lower,upper)=>assert(lower.y>=upper.bottom-1,'Form controls overlap');
  below(box.scope,box.name);below(box.instrument,box.identity);below(box.sampling,box.entry);below(box.judge,box.instrument);below(box.plan,box.judge);
  assert(box.inputSize>=12 && box.inputSize<=16,'Compact, readable rule name input');
  if(box.theme!=='dark') assert(box.colorScheme!=='dark','The light editor must inherit its theme');
  for (const key of ['selection','entry']) {
    assert(box[key].x>=box.composer.x && box[key].right<=box.composer.right+1 && box[key].y>=box.composer.y && box[key].bottom<=box.composer.bottom+1, 'Selection and input share a bounded composer');
  }
  const stacked=box.entry.y>=box.selection.bottom-1;
  if(!stacked) assert(box.entry.x>=box.selection.right-1,'Model tokens and input do not overlap');
  below(box.shortcuts,box.composer);below(box.sampling,box.shortcuts);
  assert(Math.abs(box.caption.y+box.caption.height/2-box.instrument.y)<=2,'Model title sits on the border');
  assert(box.modelTitle.right<=box.count.x && box.modelTitle.x>=box.instrument.x && box.count.right<=box.instrument.right,'Border labels stay separated and inside the card width');
  if(box.modal.width<=580) below(box.quantity,box.effort);
  else assert(box.quantity.x>=box.effort.right && Math.abs(box.quantity.y-box.effort.y)<=1,'Shared parameters align');
  return {layout:'modal',model_entry:'unified_composer',border_titles:true,composer_wrap:stacked?'stacked':'inline',shared_parameters:box.modal.width<=580?'stacked':'paired',editor_width:Math.round(box.form.width),bounded_scroll:true,theme_inherited:true};
};

// Include the border caption, which intentionally extends above the card's box.
module.exports.captureModelCard = async function captureModelCard(page, card, path) {
  await card.evaluate(el => { el.closest('.rule-form-scroll').scrollTop = Math.max(0,el.offsetTop-14); });
  const rect=await card.boundingBox();
  await page.screenshot({path,clip:{x:rect.x,y:rect.y-12,width:rect.width,height:rect.height+12},animations:'disabled'});
};
