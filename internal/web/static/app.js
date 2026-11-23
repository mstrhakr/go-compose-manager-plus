document.addEventListener('htmx:beforeRequest', () => {
  const error = document.querySelector('#request-error');
  if (error) {
    error.hidden = true;
    error.textContent = '';
  }
});

for (const event of ['htmx:responseError', 'htmx:sendError', 'htmx:timeout']) {
  document.addEventListener(event, () => {
    const error = document.querySelector('#request-error');
    if (error) {
      error.textContent = 'Unable to complete that request. Please try again.';
      error.hidden = false;
    }
  });
}

const railToggle = document.querySelector('[data-rail-toggle]');
if (railToggle) {
  const layout = railToggle.closest('.console-body');
  const notice = document.querySelector('[data-rail-notice]');
  const key = 'compose-manager-navigation-expanded';
  function reportStorageError() {
    notice.hidden = false;
    notice.textContent = 'Navigation preference could not be saved in this browser.';
  }
  function render(expanded) {
    layout.classList.toggle('navigation-expanded', expanded);
    const label = expanded ? 'Collapse navigation' : 'Expand navigation';
    railToggle.setAttribute('aria-expanded', String(expanded));
    railToggle.setAttribute('aria-label', label);
    railToggle.title = label;
    railToggle.querySelector('[aria-hidden]').textContent = expanded ? '«' : '»';
  }
  try {
    render(localStorage.getItem(key) === 'true');
  } catch {
    reportStorageError();
  }
  railToggle.hidden = false;
  railToggle.addEventListener('click', () => {
    const expanded = !layout.classList.contains('navigation-expanded');
    render(expanded);
    try {
      localStorage.setItem(key, String(expanded));
    } catch {
      reportStorageError();
    }
  });
}

const filterLink = document.querySelector('.inventory-filter-link');
if (filterLink) {
  filterLink.addEventListener('click', () => {
    document.querySelector('.inventory-filter-disclosure').open = true;
  });
}

const metricsProfile = document.querySelector('[data-metrics-profile]');
if (metricsProfile) {
  const scope = document.querySelector('#metrics-scope');
  const interval = document.querySelector('#metrics-interval');
  const storageLimit = document.querySelector('#metrics-limit');
  const presets = {
    low: ['activated', '15s'],
    medium: ['activated', '5s'],
    high: ['all', '1s'],
  };
  metricsProfile.addEventListener('change', () => {
    const preset = presets[metricsProfile.value];
    if (preset) {
      [scope.value, interval.value] = preset;
    }
  });
  for (const control of [scope, interval, storageLimit]) {
    control.addEventListener('input', () => {
      metricsProfile.value = 'custom';
    });
    control.addEventListener('change', () => {
      metricsProfile.value = 'custom';
    });
  }
}
