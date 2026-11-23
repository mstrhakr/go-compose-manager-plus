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

const metricsProfiles = [...document.querySelectorAll('[data-metrics-profile]')];
if (metricsProfiles.length) {
  const scope = document.querySelector('#metrics-scope');
  const interval = document.querySelector('#metrics-interval');
  const storageLimit = document.querySelector('#metrics-limit');
  const storageUnit = document.querySelector('[data-metrics-storage-unit]');
  const customOptions = document.querySelector('[data-metrics-custom-options]');
  const profileDescription = document.querySelector('[data-metrics-profile-description]');
  const presets = {
    low: { scope: 'activated', interval: '15s', description: 'Activated stacks, sampled every 15 seconds.' },
    medium: { scope: 'activated', interval: '5s', description: 'Activated stacks, sampled every 5 seconds.' },
    high: { scope: 'all', interval: '1s', description: 'All running containers, sampled every second.' },
    custom: { description: 'Choose a collection scope and sampling interval below.' },
  };
  const selectedProfile = () => metricsProfiles.find(control => control.checked)?.value;
  function renderProfile() {
    const profile = selectedProfile();
    if (customOptions) customOptions.hidden = profile !== 'custom';
    if (profileDescription) profileDescription.textContent = presets[profile]?.description || '';
  }
  function selectCustomProfile() {
    const custom = metricsProfiles.find(control => control.value === 'custom');
    if (custom) custom.checked = true;
    renderProfile();
  }
  function updateStorageMinimum() {
    if (!storageLimit || !storageUnit) return;
    storageLimit.min = { B: '1', KB: '0.001', MB: '0.000001', GB: '0.000000001' }[storageUnit.value] || '1';
  }
  for (const control of metricsProfiles) {
    control.addEventListener('change', () => {
      if (!control.checked) return;
      const preset = presets[control.value];
      if (preset?.scope) {
        scope.value = preset.scope;
        interval.value = preset.interval;
      }
      renderProfile();
    });
  }
  for (const control of [scope, interval, storageLimit, storageUnit]) {
    if (!control) continue;
    control.addEventListener('input', selectCustomProfile);
    control.addEventListener('change', () => {
      if (control === storageUnit) updateStorageMinimum();
      selectCustomProfile();
    });
  }
  renderProfile();
  updateStorageMinimum();
}
