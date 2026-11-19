const assert = require('node:assert/strict');

module.exports = async function checkAccountSurface(dialog) {
  const layout = await dialog.evaluate(async node => {
    await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
    const rect = el => el.getBoundingClientRect().toJSON();
    const footer = node.querySelector('.account-surface-footer');
    const body = node.querySelector('.login-history-body, .account-settings-layout, .profile-layout');
    return { id:node.id, width:innerWidth, viewportHeight:innerHeight, bounds:rect(node), heading:rect(node.querySelector('.account-surface-heading')), footer:rect(footer), body:rect(body), scrollHeight:node.scrollHeight,clientHeight:node.clientHeight, horizontalOverflow:node.scrollWidth > node.clientWidth + 1, outerScroll:node.scrollHeight > node.clientHeight + 1, activeInside:node.contains(document.activeElement) };
  });
  if (layout.outerScroll && process.env.UI_OUTPUT) await dialog.screenshot({path:require('node:path').join(process.env.UI_OUTPUT,`overflow-${layout.id}-${layout.width}.png`)});
  assert(!layout.horizontalOverflow, 'Account dialog must not overflow horizontally');
  assert(!layout.outerScroll, `Account dialog scrolls its content instead of its outer shell: ${JSON.stringify(layout)}`);
  assert(layout.bounds.left >= -1 && layout.bounds.right <= layout.width + 1);
  assert(layout.bounds.top >= 0 && layout.bounds.bottom <= layout.viewportHeight + 1, 'Account dialog must fit vertically');
  assert(layout.heading.bottom <= layout.body.top + 1);
  assert(layout.body.height >= 100 && layout.body.bottom <= layout.footer.top + 1);
  assert(layout.footer.bottom <= layout.bounds.bottom + 1, 'Footer remains in the dialog');
  assert(layout.activeInside, 'Native dialog keeps focus within its surface');
  return {surface:layout.id, width:layout.width,height:layout.bounds.height,footer_fixed:true,viewport_fit:true};
};
