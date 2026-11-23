(() => {
  const svgNS = 'http://www.w3.org/2000/svg';
  const colors = new Set(['blue', 'amber', 'purple', 'green', 'red']);
  const columnStates = new WeakMap();

  function drawChart(chart) {
    const raw = chart.dataset.chartSeries;
    const message = chart.querySelector('[data-chart-message]');
    const errorMessage = chart.querySelector('[data-chart-error]');
    const plot = chart.querySelector('[data-chart-plot]');
    const data = chart.querySelector('[data-chart-data]');
    if (!message || !errorMessage || !plot || !data) return;

    if (!raw) {
      plot.replaceChildren();
      data.replaceChildren();
      plot.hidden = true;
      data.hidden = true;
      message.hidden = false;
      errorMessage.hidden = true;
      return;
    }

    try {
      const input = JSON.parse(raw);
      if (!input || !Array.isArray(input.series) || !Number.isFinite(input.min) ||
          !Number.isFinite(input.max) || input.min >= input.max ||
          !Number.isFinite(input.max - input.min) || !input.series.length ||
          (input.xLabel !== undefined && typeof input.xLabel !== 'string') ||
          (input.yLabel !== undefined && typeof input.yLabel !== 'string')) {
        throw new Error('Invalid chart range or series');
      }
      const points = [];
      const samples = [];
      for (const series of input.series) {
        if (typeof series.label !== 'string' || !series.label.trim() ||
            !colors.has(series.color) || !Array.isArray(series.points)) {
          throw new Error('Invalid chart series');
        }
        let lastX = -Infinity;
        for (const point of series.points) {
          if (!point || !Number.isFinite(point.x) || point.x <= lastX ||
              (point.y !== null && (!Number.isFinite(point.y) || point.y < input.min || point.y > input.max))) {
            throw new Error('Invalid chart point');
          }
          lastX = point.x;
          samples.push(point);
          if (point.y !== null) points.push(point);
        }
      }
      if (!samples.length) {
        plot.replaceChildren();
        data.replaceChildren();
        plot.hidden = true;
        data.hidden = true;
        message.hidden = false;
        errorMessage.hidden = true;
        return;
      }
      const minX = samples.reduce((min, point) => Math.min(min, point.x), Infinity);
      const maxX = samples.reduce((max, point) => Math.max(max, point.x), -Infinity);
      if (!Number.isFinite(maxX - minX)) throw new Error('Invalid chart time range');
      const svg = document.createElementNS(svgNS, 'svg');
      svg.setAttribute('viewBox', '0 0 1000 200');
      svg.setAttribute('preserveAspectRatio', 'none');
      svg.setAttribute('aria-hidden', 'true');
      const legend = document.createElement('div');
      legend.className = 'chart-legend';
      const axis = document.createElement('div');
      axis.className = 'chart-axis';
      const yAxis = document.createElement('span');
      yAxis.textContent = `${input.yLabel || 'Value'}: ${input.min}–${input.max}`;
      const xAxis = document.createElement('span');
      xAxis.textContent = `${input.xLabel || 'Time'}: ${minX}–${maxX}`;
      axis.append(yAxis, xAxis);
      const details = document.createElement('details');
      const summary = document.createElement('summary');
      summary.textContent = 'View chart data';
      details.append(summary);
      const table = document.createElement('table');
      table.className = 'data-table';
      const head = table.createTHead().insertRow();
      for (const label of ['Series', input.xLabel || 'Time', input.yLabel || 'Value']) {
        const th = document.createElement('th');
        th.scope = 'col';
        th.textContent = label;
        head.append(th);
      }
      const body = table.createTBody();
      for (const series of input.series) {
        const label = document.createElement('span');
        label.className = `chart-key chart-${series.color}`;
        label.textContent = series.label;
        legend.append(label);
        let segment = [];
        const flush = () => {
          if (!segment.length) return;
          const element = document.createElementNS(svgNS, segment.length === 1 ? 'circle' : 'polyline');
          element.setAttribute('class', `chart-line chart-${series.color}`);
          if (segment.length === 1) {
            element.setAttribute('cx', segment[0][0]);
            element.setAttribute('cy', segment[0][1]);
            element.setAttribute('r', '3');
          } else {
            element.setAttribute('points', segment.map(point => point.join(',')).join(' '));
          }
          svg.append(element);
          segment = [];
        };
        for (const point of series.points) {
          const row = body.insertRow();
          for (const value of [series.label, point.x, point.y === null ? 'Missing observation' : point.y]) {
            row.insertCell().textContent = String(value);
          }
          if (point.y === null) {
            flush();
            continue;
          }
          const x = minX === maxX ? 500 : 10 + (point.x - minX) / (maxX - minX) * 980;
          const y = 190 - (point.y - input.min) / (input.max - input.min) * 180;
          segment.push([x, y]);
        }
        flush();
      }
      const tableScroll = document.createElement('div');
      tableScroll.className = 'chart-table-scroll';
      tableScroll.append(table);
      details.append(tableScroll);
      plot.replaceChildren(svg);
      data.replaceChildren(axis, legend, details);
      plot.hidden = points.length === 0;
      data.hidden = false;
      message.hidden = points.length > 0;
      errorMessage.hidden = true;
    } catch (error) {
      plot.replaceChildren();
      data.replaceChildren();
      plot.hidden = true;
      data.hidden = true;
      message.hidden = true;
      errorMessage.hidden = false;
      console.error('Unable to render chart:', error);
    }
  }

  function items(list) {
    return [...list.querySelectorAll('[data-select-item]')].filter(item =>
      item.closest('[data-select-list]') === list && !item.disabled);
  }

  function updateList(list) {
    const entries = items(list);
    const selected = entries.filter(item => item.checked).length;
    const all = list.querySelector('[data-select-all]');
    const allControl = list.querySelector('[data-select-all-control]');
    const count = list.querySelector('[data-select-count]');
    if (all) {
      if (allControl) allControl.hidden = false;
      all.disabled = entries.length === 0;
      all.checked = entries.length > 0 && selected === entries.length;
      all.indeterminate = selected > 0 && selected < entries.length;
    }
    if (count) {
      count.hidden = false;
      count.textContent = `${selected} selected`;
    }
  }

  function initializeColumns(manager) {
    const controls = manager.querySelector('[data-column-controls]');
    if (!controls || !controls.querySelector('[data-column-notice]') ||
        !controls.querySelector('[data-column-option]') || !controls.querySelector('[data-column-restore]')) return;
    const key = `compose-manager-columns:${manager.dataset.columnManager}`;
    const options = () => [...controls.querySelectorAll('[data-column-option]')];
    function reportFailure(action) {
      const notice = controls.querySelector('[data-column-notice]');
      if (!notice) return;
      notice.hidden = false;
      notice.textContent = `Column preferences could not be ${action} in this browser.`;
    }
    function apply() {
      const visible = new Set(options().filter(option => option.checked).map(option => option.value));
      for (const cell of manager.querySelectorAll('[data-column]')) {
        if (cell.closest('[data-column-manager]') === manager) cell.hidden = !visible.has(cell.dataset.column);
      }
    }
    let hidden = columnStates.get(controls);
    if (!hidden) {
      hidden = [];
      try {
        const saved = localStorage.getItem(key);
        if (saved !== null) {
          hidden = JSON.parse(saved);
          if (!Array.isArray(hidden) || !hidden.every(value => typeof value === 'string')) {
            throw new Error('Invalid column preferences');
          }
        }
      } catch (error) {
        hidden = [];
        reportFailure('loaded');
        console.error('Unable to load column preferences:', error);
      }
      columnStates.set(controls, hidden);
    }
    for (const option of options()) {
      option.checked = option.disabled || !hidden.includes(option.value);
    }
    apply();
    controls.hidden = false;
    if (controls.dataset.initialized) return;
    controls.dataset.initialized = 'true';
    function save() {
      columnStates.set(controls, options().filter(option => !option.checked).map(option => option.value));
      apply();
      try {
        localStorage.setItem(key, JSON.stringify(columnStates.get(controls)));
        const notice = controls.querySelector('[data-column-notice]');
        if (notice) notice.hidden = true;
      } catch (error) {
        reportFailure('saved');
        console.error('Unable to save column preferences:', error);
      }
    }
    controls.addEventListener('change', event => {
      if (event.target.matches('[data-column-option]')) save();
    });
    controls.addEventListener('click', event => {
      if (!(event.target instanceof Element) || !event.target.closest('[data-column-restore]')) return;
      for (const option of options()) option.checked = true;
      save();
    });
  }

  function initialize(root) {
    const charts = [...root.querySelectorAll('[data-chart]')];
    if (root.matches?.('[data-chart]')) charts.unshift(root);
    const parentChart = root.closest?.('[data-chart]');
    if (parentChart && !charts.includes(parentChart)) charts.push(parentChart);
    for (const chart of charts) drawChart(chart);
    const lists = [...root.querySelectorAll('[data-select-list]')];
    if (root.matches?.('[data-select-list]')) lists.unshift(root);
    const parentList = root.closest?.('[data-select-list]');
    if (parentList && !lists.includes(parentList)) lists.push(parentList);
    for (const list of lists) updateList(list);
    const managers = [...root.querySelectorAll('[data-column-manager]')];
    if (root.matches?.('[data-column-manager]')) managers.unshift(root);
    const parentManager = root.closest?.('[data-column-manager]');
    if (parentManager && !managers.includes(parentManager)) managers.push(parentManager);
    for (const manager of managers) initializeColumns(manager);
  }

  document.addEventListener('change', event => {
    const target = event.target;
    if (!(target instanceof HTMLInputElement)) return;
    const list = target.closest('[data-select-list]');
    if (!list) return;
    if (target.matches('[data-select-all]')) {
      for (const item of items(list)) item.checked = target.checked;
    } else if (!target.matches('[data-select-item]')) {
      return;
    }
    updateList(list);
    list.dispatchEvent(new CustomEvent('list:selectionchange', {
      bubbles: true,
      detail: { selected: items(list).filter(item => item.checked).map(item => item.value) },
    }));
  });
  document.addEventListener('DOMContentLoaded', () => initialize(document));
  document.addEventListener('htmx:afterSwap', event => initialize(event.target));
})();
