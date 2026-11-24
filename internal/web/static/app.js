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

const observabilityForm = document.querySelector('[data-observability-settings]');
if (observabilityForm) {
  observabilityForm.classList.add('settings-enhanced');
  const overallProfiles = [...observabilityForm.querySelectorAll('[data-overall-profile]')];
  const statsProfiles = [...observabilityForm.querySelectorAll('[data-metrics-profile]')];
  const logsProfiles = [...observabilityForm.querySelectorAll('[data-logs-profile]')];
  const statsScope = observabilityForm.querySelector('#metrics-scope');
  const statsInterval = observabilityForm.querySelector('#metrics-interval');
  const stateScope = observabilityForm.querySelector('[data-state-scope]');
  const logsScope = observabilityForm.querySelector('[data-logs-scope]');
  const logsDelivery = observabilityForm.querySelector('[data-logs-delivery-preset]');
  const statsCustom = observabilityForm.querySelector('[data-metrics-custom-options]');
  const logsCustom = observabilityForm.querySelector('[data-logs-custom-options]');
  const logsPresetValues = observabilityForm.querySelector('[data-logs-preset-values]');
  const logsDeliveryText = observabilityForm.querySelector('[data-logs-current-delivery]');
  const logsNotCollecting = observabilityForm.querySelector('[data-logs-not-collecting]');
  const metricsLimit = observabilityForm.querySelector('#metrics-limit');
  const metricsUnit = observabilityForm.querySelector('[data-metrics-storage-unit]');
  const logsLimit = observabilityForm.querySelector('#logs-limit');
  const logsUnit = observabilityForm.querySelector('[data-logs-storage-unit]');
  const statsPresets = Object.fromEntries([...observabilityForm.querySelectorAll('[data-metrics-preset]')]
    .map(values => [values.dataset.metricsPreset, { scope: values.dataset.scope, interval: values.dataset.interval }]));
  const control = (controls, value) => controls.find(item => item.value === value);
  const checked = controls => controls.find(item => item.checked)?.value;
  const setChecked = (controls, value) => {
    const item = control(controls, value);
    if (item) item.checked = true;
  };
  const stateSavedScope = observabilityForm.querySelector('[name="state_saved_scope"]');
  const statsSavedProfile = observabilityForm.querySelector('[name="stats_saved_profile"]');
  const statsSavedScope = observabilityForm.querySelector('[name="stats_saved_scope"]');
  const logsSavedProfile = observabilityForm.querySelector('[name="logs_saved_profile"]');
  const restoreStats = observabilityForm.querySelector('[name="restore_stats"]');
  const restoreState = observabilityForm.querySelector('[name="restore_state"]');
  const restoreLogs = observabilityForm.querySelector('[name="restore_logs"]');
  const overallRestore = observabilityForm.querySelector('[name="overall_restore"]');
  observabilityForm.querySelector('[name="client_managed"]').value = 'true';
  let previousOverall = checked(overallProfiles);

  function chooseCustomOverall() {
    setChecked(overallProfiles, 'custom');
    overallRestore.value = 'false';
    previousOverall = 'custom';
  }

  function renderStats() {
    const profile = checked(statsProfiles);
    statsSavedScope.value = statsScope.value;
    if (statsCustom) statsCustom.hidden = profile === 'off';
    if (statsInterval) statsInterval.readOnly = profile !== 'custom';
    statsScope.disabled = profile !== 'custom';
    observabilityForm.querySelector('[data-stats-not-collecting]').hidden = profile !== 'off';
    for (const item of observabilityForm.querySelectorAll('[data-metrics-scope-control]')) {
      item.hidden = profile !== 'custom';
    }
  }

  function renderLogs() {
    const profile = checked(logsProfiles);
    if (logsCustom) logsCustom.hidden = profile !== 'custom';
    const enabled = profile !== 'off';
    if (logsPresetValues) logsPresetValues.hidden = !enabled;
    if (logsNotCollecting) logsNotCollecting.hidden = enabled;
    if (!logsDeliveryText || !enabled) return;
    const delivery = profile === 'custom' ? logsDelivery.value : profile;
    const scope = logsScope.value === 'all' ? 'All containers' : 'Activated stacks';
    if (!delivery) {
      logsDeliveryText.textContent = 'Current delivery: choose and save Low, Medium or High before using Custom.';
      return;
    }
    const description = delivery === 'high'
      ? `${scope} · live follow`
      : `${scope} · periodic backlog every ${delivery === 'medium' ? '30' : '60'} seconds`;
    logsDeliveryText.textContent = `Current delivery: ${description}`;
  }

  function updateStorageMinimum(input, unit) {
    if (!input || !unit) return;
    input.min = { B: '1', KB: '0.001', MB: '0.000001', GB: '0.000000001' }[unit.value] || '1';
  }

  function applyOverall(profile) {
    if (profile === 'off') {
      if (previousOverall !== 'off') {
        restoreStats.value = String(checked(statsProfiles) !== 'off');
        restoreState.value = String(stateScope.value !== 'off');
        restoreLogs.value = String(checked(logsProfiles) !== 'off');
      }
      setChecked(statsProfiles, 'off');
      stateScope.value = 'off';
      setChecked(logsProfiles, 'off');
    } else if (profile === 'custom' && previousOverall === 'off') {
      overallRestore.value = 'true';
      setChecked(statsProfiles, restoreStats.value === 'true' ? statsSavedProfile.value : 'off');
      stateScope.value = restoreState.value === 'true' ? stateSavedScope.value : 'off';
      setChecked(logsProfiles, restoreLogs.value === 'true' ? logsSavedProfile.value : 'off');
    } else if (['low', 'medium', 'high'].includes(profile)) {
      const preset = statsPresets[profile];
      setChecked(statsProfiles, profile);
      statsSavedProfile.value = profile;
      if (preset) {
        statsScope.value = preset.scope;
        statsInterval.value = preset.interval;
      }
      const stateChoice = profile === 'high' ? 'all' : 'activated';
      stateScope.value = stateChoice;
      stateSavedScope.value = stateChoice;
      setChecked(logsProfiles, profile);
      logsSavedProfile.value = profile;
      logsScope.value = profile === 'high' ? 'all' : 'activated';
      logsDelivery.value = profile;
    }
    previousOverall = profile;
    renderStats();
    renderLogs();
  }

  for (const item of overallProfiles) {
    item.addEventListener('change', () => {
      if (!item.checked) return;
      applyOverall(item.value);
    });
  }
  for (const item of statsProfiles) {
    item.addEventListener('change', () => {
      if (!item.checked) return;
      const preset = statsPresets[item.value];
      if (item.value !== 'off') statsSavedProfile.value = item.value;
      if (preset) {
        statsScope.value = preset.scope;
        statsInterval.value = preset.interval;
      }
      chooseCustomOverall();
      renderStats();
    });
  }
  for (const item of logsProfiles) {
    item.addEventListener('change', () => {
      if (!item.checked) return;
      if (item.value !== 'off') logsSavedProfile.value = item.value;
      if (item.value === 'low' || item.value === 'medium') {
        logsScope.value = 'activated';
        logsDelivery.value = item.value;
      } else if (item.value === 'high') {
        logsScope.value = 'all';
        logsDelivery.value = 'high';
      }
      chooseCustomOverall();
      renderLogs();
    });
  }
  for (const item of [statsScope, statsInterval, metricsLimit, metricsUnit, stateScope, logsScope, logsLimit, logsUnit]) {
    if (!item) continue;
    item.addEventListener('input', () => {
      if (item === stateScope && item.value !== 'off') stateSavedScope.value = item.value;
      if ([statsScope, statsInterval, metricsLimit, metricsUnit].includes(item)) {
        statsSavedProfile.value = 'custom';
        if (checked(statsProfiles) !== 'off') setChecked(statsProfiles, 'custom');
      }
      if ([logsScope, logsLimit, logsUnit].includes(item) && logsDelivery.value) {
        logsSavedProfile.value = 'custom';
        if (checked(logsProfiles) !== 'off') setChecked(logsProfiles, 'custom');
      }
      chooseCustomOverall();
    });
    item.addEventListener('change', () => {
      if (item === metricsUnit) updateStorageMinimum(metricsLimit, metricsUnit);
      if (item === logsUnit) updateStorageMinimum(logsLimit, logsUnit);
      if (item === stateScope && item.value !== 'off') stateSavedScope.value = item.value;
      if ([statsScope, statsInterval, metricsLimit, metricsUnit].includes(item)) {
        statsSavedProfile.value = 'custom';
        if (checked(statsProfiles) !== 'off') setChecked(statsProfiles, 'custom');
      }
      if ([logsScope, logsLimit, logsUnit].includes(item) && logsDelivery.value) {
        logsSavedProfile.value = 'custom';
        if (checked(logsProfiles) !== 'off') setChecked(logsProfiles, 'custom');
      }
      renderStats();
      renderLogs();
      chooseCustomOverall();
    });
  }
  renderStats();
  renderLogs();
  updateStorageMinimum(metricsLimit, metricsUnit);
  updateStorageMinimum(logsLimit, logsUnit);
}
