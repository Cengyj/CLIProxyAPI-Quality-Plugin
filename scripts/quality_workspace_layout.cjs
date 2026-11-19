// Check the paired panels and their scroll boundaries in standalone and host frames.
const assert = require('node:assert/strict');

module.exports = async function checkQualityWorkspace(workspace) {
  const geometry = await workspace.evaluate(async node => {
    await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
    const rect = el => { const r = el.getBoundingClientRect(); return { top: r.top, bottom: r.bottom, left: r.left, width: r.width, height: r.height }; };
    const panels = [...node.querySelectorAll('.quality-panel')].map(panel => {
      const list = panel.querySelector('.quality-rule-list, .quality-run-list');
      return { ...rect(panel), header: rect(panel.querySelector('.quality-panel-heading')), footer: rect(panel.querySelector(':scope > footer')), list: { ...rect(list), scrollHeight: list.scrollHeight, clientHeight: list.clientHeight } };
    });
    return { panels, workspace: rect(node), viewport: innerWidth, documentWidth: document.documentElement.scrollWidth };
  });
  const [rules, records] = geometry.panels;
  assert.equal(geometry.panels.length, 2);
  assert(geometry.documentWidth <= geometry.viewport + 1, 'Quality workspace must not overflow horizontally');
  const paired = Math.abs(rules.top - records.top) < 1;
  if (paired) {
    assert(Math.abs(rules.bottom - records.bottom) < 1, 'Desktop panels must share the same bottom edge');
    assert(Math.abs(rules.footer.top - records.footer.top) < 1, 'Panel footers must align');
    assert(Math.abs(rules.header.bottom - records.header.bottom) < 1, 'Panel headings must align');
    assert(Math.abs(records.bottom - geometry.workspace.bottom) < 1);
  } else {
    assert(records.top >= rules.bottom + 12, 'Narrow panels must stack with clear spacing');
  }
  for (const panel of geometry.panels) {
    assert(panel.list.height >= 120, 'Each list needs useful reading space');
    assert(panel.list.bottom <= panel.footer.top + 1, 'Content must stay above the fixed footer');
    assert(panel.footer.bottom <= panel.bottom + 1, 'Footer must stay inside its panel');
  }
  return { layout: paired ? 'paired' : 'stacked', width: geometry.viewport, panel_heights: geometry.panels.map(p => p.height), bottoms_aligned: paired };
};
