// Verify the evaluation panels, fixed controls and visible answer pairs.
const assert = require('node:assert/strict');

module.exports = async function checkManualWorkspace(workspace) {
  const geometry = await workspace.evaluate(async node => {
    await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
    const rect = el => { const r = el.getBoundingClientRect(); return { top:r.top, bottom:r.bottom, width:r.width, height:r.height }; };
    const visible = el => el && el.getClientRects().length > 0;
    const panels = [...node.children].filter(visible).map(panel => {
      const content = panel.id === 'manual-setup' ? panel.querySelector('form') : [...panel.querySelectorAll(':scope > section')].find(visible);
      return { id:panel.id || 'answers', ...rect(panel), header:visible(panel.querySelector(':scope > header')) ? rect(panel.querySelector(':scope > header')) : null, footer:rect(panel.querySelector(':scope > footer')), content:rect(content) };
    });
    const samples = [...node.querySelectorAll('.question-test-results .sample')].filter(visible).map(sample => ({...rect(sample),footer:sample.querySelector('.sample-footer') ? rect(sample.querySelector('.sample-footer')) : null}));
    return {panels,samples,viewport:innerWidth,documentWidth:document.documentElement.scrollWidth};
  });
  assert(geometry.documentWidth <= geometry.viewport + 1, 'Manual workspace must fit horizontally');
  assert(geometry.panels.length >= 1);
  if (geometry.panels.length === 2) {
    const [setup,answers] = geometry.panels;
    for (const edge of ['top','bottom']) assert(Math.abs(setup[edge]-answers[edge])<1, `Manual panel ${edge} edges must align`);
    assert(Math.abs(setup.footer.top-answers.footer.top)<1, 'Manual footer dividers must align');
    assert(Math.abs(setup.header.bottom-answers.header.bottom)<1, 'Manual heading dividers must align');
  }
  for (const panel of geometry.panels) {
    assert(panel.content.height >= 160, 'Each manual panel needs useful reading space');
    assert(panel.content.bottom <= panel.footer.top + 1, 'Scrolling content must stop above its footer');
    assert(panel.footer.bottom <= panel.bottom + 1, 'Manual footer must remain within its panel');
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
