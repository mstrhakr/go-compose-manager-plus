(() => {
  let opener = null;
  const narrow = window.matchMedia('(max-width: 620px)');
  const host = () => document.getElementById('resource-details-host');
  const notice = () => document.querySelector('[data-resource-details-request]');
  const drawer = () => host()?.querySelector('[data-resource-details]');

  function report(message, failed = false) {
    const element = notice();
    if (!element) return;
    element.querySelector('[data-resource-details-message]').textContent = message;
    element.hidden = !message;
    element.setAttribute('role', failed ? 'alert' : 'status');
    const feedback = drawer()?.querySelector('[data-resource-details-feedback]');
    if (feedback) {
      feedback.textContent = failed ? message : '';
      feedback.hidden = !failed;
    }
  }

  function highlight() {
    const key = drawer()?.dataset.resourceKey;
    for (const link of document.querySelectorAll('.resource-detail-link')) {
      if (key && new URL(link.href).searchParams.get('details') === key) link.setAttribute('aria-current', 'true');
      else link.removeAttribute('aria-current');
    }
  }

  function close() {
    host()?.dispatchEvent(new CustomEvent('htmx:abort', { bubbles: true, detail: {} }));
    drawer()?.close();
    drawer()?.remove();
    report('');
    highlight();
    if (opener?.isConnected) opener.focus({ preventScroll: true });
    opener = null;
  }

  function setMode(dialog) {
    dialog.close();
    if (narrow.matches) dialog.showModal();
    else dialog.show();
  }

  function initialize() {
    const dialog = drawer();
    if (!dialog || dialog.dataset.initialized) return;
    dialog.dataset.initialized = 'true';
    setMode(dialog);
    dialog.addEventListener('cancel', event => { event.preventDefault(); close(); });
    const tabs = [...dialog.querySelectorAll('[data-resource-tab]')];
    dialog.querySelector('.resource-details-tabs').setAttribute('role', 'tablist');
    function select(tab, focus = false) {
      for (const candidate of tabs) {
        const selected = candidate === tab;
        candidate.setAttribute('role', 'tab');
        candidate.setAttribute('aria-controls', `resource-panel-${candidate.dataset.resourceTab}`);
        candidate.setAttribute('aria-selected', String(selected));
        candidate.tabIndex = selected ? 0 : -1;
      }
      for (const panel of dialog.querySelectorAll('[data-resource-panel]')) {
        panel.setAttribute('role', 'tabpanel');
        panel.hidden = panel.dataset.resourcePanel !== tab.dataset.resourceTab;
        panel.tabIndex = 0;
      }
      if (focus) tab.focus();
    }
    for (const [index, tab] of tabs.entries()) {
      tab.addEventListener('click', event => { event.preventDefault(); select(tab); });
      tab.addEventListener('keydown', event => {
        if (!['ArrowLeft', 'ArrowRight', 'Home', 'End', ' '].includes(event.key)) return;
        event.preventDefault();
        const next = event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1 :
          event.key === ' ' ? index : (index + (event.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length;
        select(tabs[next], true);
      });
    }
    select(tabs[0]);
    dialog.querySelector('h2').focus({ preventScroll: true });
    highlight();
  }

  document.addEventListener('click', event => {
    if (event.target instanceof Element && event.target.closest('[data-resource-details-open]') &&
        (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey || event.button !== 0)) {
      event.stopPropagation();
    }
  }, true);
  document.addEventListener('click', event => {
    if (!(event.target instanceof Element)) return;
    if (event.target.closest('[data-resource-details-close]')) {
      event.preventDefault();
      close();
      return;
    }
    const link = event.target.closest('[data-resource-details-open]');
    if (link && !event.ctrlKey && !event.metaKey && !event.shiftKey && !event.altKey && event.button === 0) {
      if (!link.closest('[data-resource-details]')) opener = link;
      return;
    }
    if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey || event.button !== 0) return;
    const row = event.target.closest('[data-resource-row]');
    if (row && !event.target.closest('a,button,input,label,select,textarea') && !window.getSelection()?.toString()) {
      row.querySelector('[data-resource-details-open]')?.click();
    }
  });
  document.addEventListener('keydown', event => {
    if (event.key === 'Escape' && !event.isComposing && (drawer() || (notice() && !notice().hidden))) {
      event.preventDefault();
      close();
    }
  });
  document.addEventListener('htmx:beforeRequest', event => {
    if (event.detail.target !== host()) return;
    if (!event.detail.elt.closest('[data-resource-details]')) opener = event.detail.elt;
    report('Loading details...');
  });
  document.addEventListener('htmx:afterSwap', event => {
    if (event.detail.target !== host()) return;
    report('');
    initialize();
  });
  for (const name of ['htmx:responseError', 'htmx:sendError', 'htmx:timeout']) {
    document.addEventListener(name, event => {
      if (event.detail.target !== host()) return;
      report('Details could not be loaded. Try again or refresh inventory.', true);
    });
  }
  narrow.addEventListener('change', () => { if (drawer()) setMode(drawer()); });
  document.addEventListener('DOMContentLoaded', initialize);
})();
