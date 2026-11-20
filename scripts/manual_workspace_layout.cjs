// Verify the reading area, panel controls and visible answer pairs.
const assert = require('node:assert/strict');

module.exports = async function checkManualWorkspace(workspace) {
  const geometry = await workspace.evaluate(async node => {
    await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
    const rect = el => { const r = el.getBoundingClientRect(); return { top:r.top, bottom:r.bottom, width:r.width, height:r.height }; };
    const visible = el => el && el.getClientRects().length > 0;
    const visibleRect = el => visible(el) ? rect(el) : null;
    const panels = [...node.children].filter(visible).map(panel => {
      const content = panel.id === 'manual-setup' ? panel.querySelector('form') : [...panel.querySelectorAll(':scope > section')].find(visible);
      return { id:content.id, ...rect(panel), header:visibleRect(panel.querySelector(':scope > header')), footer:visibleRect(panel.querySelector(':scope > footer')), content:rect(content) };
    });
    const samples = [...node.querySelectorAll('.question-test-results .sample')].filter(visible).map(sample => ({...rect(sample),footer:sample.querySelector('.sample-footer') ? rect(sample.querySelector('.sample-footer')) : null}));
    const emptyTitle = visibleRect(node.querySelector('#run-empty h3'));
    return {panels,samples,emptyTitle,viewport:innerWidth,documentWidth:document.documentElement.scrollWidth};
  });
  assert(geometry.documentWidth <= geometry.viewport + 1, 'Manual workspace must fit horizontally');
  assert(geometry.panels.length >= 1);
  if (geometry.panels.length === 2) {
    const [setup,answers] = geometry.panels;
    assert(Math.abs(setup.top-answers.top)<1, 'Manual panels must start together');
    assert(Math.abs(setup.header.bottom-answers.header.bottom)<1, 'Manual heading dividers must align');
    assert(Math.abs(setup.bottom-answers.bottom)<1, 'Both panels must share a bottom edge in the desktop workspace');
    if (answers.footer) assert(Math.abs(setup.footer.top-answers.footer.top)<1, 'The two action bars must line up');
    if (geometry.emptyTitle) {
      const promptCenter = (geometry.emptyTitle.top + geometry.emptyTitle.bottom) / 2;
      const readingCenter = (answers.content.top + answers.content.bottom) / 2;
      assert(Math.abs(promptCenter-readingCenter)<55, 'The empty answer prompt must stay near the center of the reading area');
    }
  }
  for (const panel of geometry.panels) {
    assert(panel.content.height >= 160, 'Each manual panel needs useful reading space');
    if (panel.id === 'manual-results' && geometry.emptyTitle) {
      assert.equal(panel.footer, null, 'An empty current answer should have no action bar');
      assert(Math.abs(panel.content.bottom-panel.bottom) <= 2, 'The empty reader should end after the prompt');
    } else {
      assert(panel.footer, 'A populated panel must keep its actions or pager available');
      assert(Math.abs(panel.content.bottom-panel.footer.top) <= 1, 'The footer must immediately follow the reading area');
      assert(Math.abs(panel.footer.bottom-panel.bottom) <= 2, 'The footer must remain at the bottom of its panel');
    }
  }
  let alignedPairs = 0;
  for (let i=0;i<geometry.samples.length;i++) for(let j=i+1;j<geometry.samples.length;j++) {
    const a=geometry.samples[i],b=geometry.samples[j];
    if(Math.abs(a.top-b.top)>=1) continue;
    assert(Math.abs(a.bottom-b.bottom)<1, 'Side-by-side answers must share the same bottom edge');
    if(a.footer && b.footer) assert(Math.abs(a.footer.top-b.footer.top)<1, 'Answer action dividers must align');
    alignedPairs++;
  }
  return {width:geometry.viewport,panels:geometry.panels.length,heights:geometry.panels.map(p=>p.height),aligned_answer_pairs:alignedPairs};
};
