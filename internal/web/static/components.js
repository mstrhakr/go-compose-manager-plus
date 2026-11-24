(() => {
  const svgNS = 'http://www.w3.org/2000/svg';
  const colors = new Set(['blue', 'amber', 'purple', 'green', 'red']);
  const columnStates = new WeakMap();
  const chartSelections = new WeakMap();
  const chartTooltips = new WeakMap();
  const timelineSelections = new WeakMap();

  function formatChartTime(value) {
    if (Math.abs(value) < 100000000000) return String(value);
    return new Date(value).toLocaleString();
  }

  function drawChart(chart) {
    chartSelections.delete(chart);
    const raw = chart.dataset.chartSeries;
    const message = chart.querySelector('[data-chart-message]');
    const errorMessage = chart.querySelector('[data-chart-error]');
    const plot = chart.querySelector('[data-chart-plot]');
    const data = chart.querySelector('[data-chart-data]');
    if (!message || !errorMessage || !plot || !data) return;

    if (!raw) {
      chartTooltips.delete(chart);
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
          (input.yLabel !== undefined && typeof input.yLabel !== 'string') ||
          ((input.xMin !== undefined || input.xMax !== undefined) &&
            (!Number.isFinite(input.xMin) || !Number.isFinite(input.xMax) || input.xMin >= input.xMax)) ||
          (input.selectedTime !== undefined && !Number.isFinite(input.selectedTime))) {
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
              (input.xMin !== undefined && (point.x < input.xMin || point.x > input.xMax)) ||
              (point.y !== null && (!Number.isFinite(point.y) || point.y < input.min || point.y > input.max))) {
            throw new Error('Invalid chart point');
          }
          lastX = point.x;
          samples.push(point);
          if (point.y !== null) points.push(point);
        }
      }
      if (!samples.length) {
        chartTooltips.delete(chart);
        plot.replaceChildren();
        data.replaceChildren();
        plot.hidden = true;
        data.hidden = true;
        message.hidden = false;
        errorMessage.hidden = true;
        return;
      }
      const minX = input.xMin ?? samples.reduce((min, point) => Math.min(min, point.x), Infinity);
      const maxX = input.xMax ?? samples.reduce((max, point) => Math.max(max, point.x), -Infinity);
      if (!Number.isFinite(maxX - minX)) throw new Error('Invalid chart time range');
      const svg = document.createElementNS(svgNS, 'svg');
      svg.setAttribute('viewBox', '0 0 1000 200');
      svg.setAttribute('preserveAspectRatio', 'none');
      svg.setAttribute('role', 'slider');
      svg.setAttribute('tabindex', '0');
      svg.setAttribute('aria-label', `${input.yLabel || 'Value'} over ${input.xLabel || 'time'}. Use arrow keys to select a time.`);
      svg.setAttribute('aria-valuemin', String(minX));
      svg.setAttribute('aria-valuemax', String(maxX));
      const interactionSurface = document.createElementNS(svgNS, 'rect');
      interactionSurface.setAttribute('class', 'chart-interaction-surface');
      interactionSurface.setAttribute('x', '10');
      interactionSurface.setAttribute('y', '10');
      interactionSurface.setAttribute('width', '980');
      interactionSurface.setAttribute('height', '180');
      interactionSurface.setAttribute('fill', 'transparent');
      interactionSurface.setAttribute('pointer-events', 'all');
      interactionSurface.setAttribute('aria-hidden', 'true');
      svg.append(interactionSurface);
      const legend = document.createElement('div');
      legend.className = 'chart-series-list';
      legend.setAttribute('role', 'group');
      legend.setAttribute('aria-label', 'Toggle chart series');
      const seriesDetails = document.createElement('details');
      seriesDetails.className = 'chart-series-details';
      const seriesSummary = document.createElement('summary');
      const visibleSeries = new Set(input.series.map((_, index) => index));
      const seriesElements = new Map();
      seriesSummary.textContent = `${visibleSeries.size} of ${input.series.length} series`;
      seriesDetails.append(seriesSummary, legend);
      seriesDetails.addEventListener('click', event => {
        if (!(event.target instanceof Element)) return;
        const button = event.target.closest('button[data-series-index]');
        if (!button || !legend.contains(button)) return;
        const index = Number(button.dataset.seriesIndex);
        const visible = visibleSeries.has(index);
        if (visible) visibleSeries.delete(index);
        else visibleSeries.add(index);
        button.setAttribute('aria-pressed', String(!visible));
        for (const element of seriesElements.get(index) || []) {
          element.style.display = visible ? 'none' : '';
        }
        seriesSummary.textContent = `${visibleSeries.size} of ${input.series.length} series`;
      });
      const axis = document.createElement('div');
      axis.className = 'chart-axis';
      const compact = chart.closest('.observability-charts') !== null;
      const numberFormat = new Intl.NumberFormat(undefined, { maximumSignificantDigits: 4 });
      const formatValue = value => compact ? numberFormat.format(value) : String(value);
      const yAxis = document.createElement('span');
      yAxis.textContent = `${input.yLabel || 'Value'}: ${formatValue(input.min)}–${formatValue(input.max)}`;
      const xAxis = document.createElement('span');
      xAxis.textContent = `${input.xLabel || 'Time'}: ${formatChartTime(minX)}–${formatChartTime(maxX)}`;
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
      for (let seriesIndex = 0; seriesIndex < input.series.length; seriesIndex++) {
        const series = input.series[seriesIndex];
        const label = document.createElement('button');
        label.type = 'button';
        label.className = `chart-key chart-${series.color}`;
        label.dataset.seriesIndex = String(seriesIndex);
        label.setAttribute('aria-pressed', 'true');
        label.title = series.label;
        label.textContent = series.label;
        legend.append(label);
        let segment = [];
        const flush = () => {
          if (!segment.length) return;
          const element = document.createElementNS(svgNS, segment.length === 1 ? 'circle' : 'polyline');
          element.setAttribute('class', `chart-line chart-${series.color}`);
          element.setAttribute('data-series-index', String(seriesIndex));
          if (segment.length === 1) {
            element.setAttribute('cx', segment[0][0]);
            element.setAttribute('cy', segment[0][1]);
            element.setAttribute('r', '3');
          } else {
            element.setAttribute('points', segment.map(point => point.join(',')).join(' '));
          }
          svg.append(element);
          const elements = seriesElements.get(seriesIndex) || [];
          elements.push(element);
          seriesElements.set(seriesIndex, elements);
          segment = [];
        };
        for (const point of series.points) {
          const row = body.insertRow();
          for (const value of [series.label, formatChartTime(point.x), point.y === null ? 'Missing observation' : point.y]) {
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
      const cursor = document.createElementNS(svgNS, 'line');
      cursor.setAttribute('class', 'chart-cursor');
      cursor.setAttribute('y1', '4');
      cursor.setAttribute('y2', '196');
      cursor.setAttribute('aria-hidden', 'true');
      svg.append(cursor);
      const tooltip = document.createElement('div');
      tooltip.className = 'chart-tooltip';
      tooltip.setAttribute('role', 'status');
      tooltip.hidden = true;
      chartTooltips.set(chart, () => { tooltip.hidden = true; });
      const tableScroll = document.createElement('div');
      tableScroll.className = 'chart-table-scroll';
      tableScroll.append(table);
      details.append(tableScroll);
      plot.replaceChildren(svg, tooltip);
      details.className = 'chart-data-details';
      data.replaceChildren(axis, seriesDetails, details);
      plot.hidden = points.length === 0;
      data.hidden = false;
      message.hidden = points.length > 0;
      errorMessage.hidden = true;
      const timeSpan = maxX - minX;
      const toChartX = time => timeSpan === 0 ? 500 : 10 + (time - minX) / timeSpan * 980;
      let selectedTime = input.selectedTime ?? maxX;
      function drawCursor(time) {
        const visible = Number.isFinite(time) && time >= minX && time <= maxX;
        cursor.setAttribute('x1', visible ? String(toChartX(time)) : '0');
        cursor.setAttribute('x2', visible ? String(toChartX(time)) : '0');
        cursor.style.display = visible ? '' : 'none';
        svg.setAttribute('aria-valuenow', String(time));
      }
      function setCursor(time) {
        selectedTime = time;
        drawCursor(time);
      }
      chartSelections.set(chart, setCursor);
      setCursor(selectedTime);
      const announce = time => svg.dispatchEvent(new CustomEvent('observability:timechange', {
        bubbles: true,
        detail: { time: Math.round(time) },
      }));
      const timeAtPointer = event => {
        const bounds = svg.getBoundingClientRect();
        const ratio = Math.max(0, Math.min(1, (event.clientX - bounds.left) / bounds.width));
        return timeSpan === 0 ? minX : minX + ratio * timeSpan;
      };
      function showTooltip(time, event) {
        const when = document.createElement('strong');
        when.textContent = formatChartTime(time);
        const entries = [when];
        const nearest = input.series.map((series, index) => {
          if (!visibleSeries.has(index)) return null;
          let selected = series.points[0];
          for (const point of series.points) {
            if (Math.abs(point.x - time) < Math.abs(selected.x - time)) selected = point;
          }
          return { series, point: selected };
        }).filter(Boolean);
        for (const { series, point } of nearest.slice(0, 10)) {
          const entry = document.createElement('span');
          entry.className = `chart-tooltip-entry chart-${series.color}`;
          entry.textContent = `${series.label}: ${point.y === null ? 'Missing observation' : formatValue(point.y)}`;
          entries.push(entry);
        }
        if (nearest.length > 10) {
          const more = document.createElement('span');
          more.textContent = `${nearest.length - 10} more series`;
          entries.push(more);
        }
        tooltip.replaceChildren(...entries);
        tooltip.hidden = false;
        if (event) {
          const bounds = svg.getBoundingClientRect();
          tooltip.style.left = `${Math.max(4, Math.min(bounds.width - 180, event.clientX - bounds.left + 10))}px`;
          tooltip.style.top = `${Math.max(4, event.clientY - bounds.top + 10)}px`;
        }
      }
      svg.addEventListener('pointermove', event => {
        const time = timeAtPointer(event);
        drawCursor(time);
        showTooltip(time, event);
      });
      svg.addEventListener('pointerleave', event => {
        if (event.pointerType === 'touch') return;
        drawCursor(selectedTime);
        tooltip.hidden = true;
      });
      svg.addEventListener('click', event => {
        const time = timeAtPointer(event);
        announce(time);
        showTooltip(time, event);
      });
      svg.addEventListener('keydown', event => {
        if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
        event.preventDefault();
        const times = [...new Set(samples.map(point => point.x))].sort((a, b) => a - b);
        if (!times.length) return;
        let index;
        if (event.key === 'Home') index = 0;
        else if (event.key === 'End') index = times.length - 1;
        else if (event.key === 'ArrowRight') {
          index = times.findIndex(time => time > selectedTime);
          if (index < 0) index = times.length - 1;
        } else {
          index = 0;
          for (let candidate = times.length - 1; candidate >= 0; candidate--) {
            if (times[candidate] < selectedTime) {
              index = candidate;
              break;
            }
          }
        }
        announce(times[index]);
        showTooltip(times[index]);
      });
    } catch (error) {
      chartTooltips.delete(chart);
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

  function initializeTimeline(timeline) {
    for (const span of timeline.querySelectorAll('[data-timeline-span]')) {
      const start = Number(span.dataset.spanStart);
      const height = Number(span.dataset.spanHeight);
      if (!Number.isFinite(start) || !Number.isFinite(height) ||
          start < 0 || start > 100 || height <= 0 || start + height > 100.001) continue;
      span.style.bottom = `${start}%`;
      span.style.height = `${Math.min(height, 100 - start)}%`;
      span.hidden = false;
    }
    if (timeline.dataset.timelineReady === 'true') return;
    const slider = timeline.querySelector('[data-timeline-slider]');
    const output = timeline.querySelector('[data-timeline-output]');
    const caption = timeline.querySelector('[data-timeline-caption]');
    const start = Number(timeline.dataset.start);
    const end = Number(timeline.dataset.end);
    if (!slider || !output || !Number.isFinite(start) || !Number.isFinite(end) || start >= end) return;
    const step = Number(slider.step) || 1000;
    const format = time => {
      return new Date(time).toLocaleString(undefined, {
        month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit', second: '2-digit', timeZoneName: 'short',
        fractionalSecondDigits: time % 1000 ? 3 : undefined,
      });
    };
    const setTime = time => {
      const selected = Math.round(Math.max(start, Math.min(end, time)));
      const sliderTime = Math.max(start, Math.min(end,
        start + Math.round((selected - start) / step) * step));
      slider.value = String(sliderTime);
      timeline.dataset.selected = String(selected);
      const label = format(selected);
      const vertical = timeline.hasAttribute('data-timeline-vertical');
      output.value = vertical ? new Date(selected).toLocaleTimeString(undefined, {
        hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
      }) : label;
      output.textContent = output.value;
      output.title = label;
      if (caption) caption.title = label;
      slider.setAttribute('aria-valuetext', label);
      const position = 100 - (selected - start) / (end - start) * 100;
      if (vertical) output.style.top = `clamp(12px, ${position}%, calc(100% - 12px))`;
      const cursor = timeline.querySelector('[data-timeline-cursor]');
      if (cursor) {
        cursor.style.top = `${position}%`;
        cursor.hidden = false;
      }
      for (const marker of timeline.querySelectorAll('[data-timeline-select]')) {
        const eventTime = Number(marker.dataset.timelineSelect);
        const selectedMarker = marker.hasAttribute('data-timeline-marker')
          ? Math.abs(eventTime - selected) <= step / 2
          : eventTime === selected;
        if (selectedMarker) marker.setAttribute('aria-current', 'time');
        else marker.removeAttribute('aria-current');
      }
      return selected;
    };
    const announce = () => {
      const time = Number(slider.value);
      if (!Number.isFinite(time)) return;
      setTime(time);
      timeline.dispatchEvent(new CustomEvent('observability:timechange', {
        bubbles: true,
        detail: { time: Math.round(time) },
      }));
    };
    slider.addEventListener('input', announce);
    if (timeline.hasAttribute('data-timeline-vertical')) {
      slider.addEventListener('keydown', event => {
        if (!['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
        event.preventDefault();
        let time = Number(slider.value);
        if (event.key === 'Home') time = start;
        else if (event.key === 'End') time = end;
        else time += ['ArrowUp', 'ArrowRight'].includes(event.key) ? step : -step;
        slider.value = String(Math.max(start, Math.min(end, time)));
        announce();
      });
    }
    timeline.addEventListener('click', event => {
      if (!(event.target instanceof Element)) return;
      const button = event.target.closest('[data-timeline-select]');
      if (!button || !timeline.contains(button)) return;
      const time = Number(button.dataset.timelineSelect);
      if (!Number.isFinite(time)) return;
      const selected = setTime(time);
      timeline.dispatchEvent(new CustomEvent('observability:timechange', {
        bubbles: true,
        detail: { time: selected },
      }));
    });
    timelineSelections.set(timeline, setTime);
    timeline.dataset.timelineReady = 'true';
    const latest = timeline.querySelector('[data-timeline-latest]');
    if (latest) latest.hidden = false;
    const initial = Number(timeline.dataset.selected);
    setTime(Number.isFinite(initial) ? initial : end);
    for (const marker of timeline.querySelectorAll('[data-timeline-marker]')) {
      const position = Number(marker.dataset.position);
      if (Number.isFinite(position)) {
        const bounded = Math.max(0, Math.min(100, position));
        if (timeline.hasAttribute('data-timeline-vertical')) marker.style.top = `${100 - bounded}%`;
        else marker.style.left = `${bounded}%`;
      }
    }
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
    const timelines = [...root.querySelectorAll('[data-timeline]')];
    if (root.matches?.('[data-timeline]')) timelines.unshift(root);
    const parentTimeline = root.closest?.('[data-timeline]');
    if (parentTimeline && !timelines.includes(parentTimeline)) timelines.push(parentTimeline);
    for (const timeline of timelines) initializeTimeline(timeline);
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
  document.addEventListener('observability:timechange', event => {
    const time = event.detail?.time;
    if (!Number.isFinite(time)) return;
    let selectedTime = time;
    for (const timeline of document.querySelectorAll('[data-timeline]')) {
      selectedTime = timelineSelections.get(timeline)?.(time) ?? selectedTime;
    }
    for (const chart of document.querySelectorAll('[data-chart]')) {
      chartSelections.get(chart)?.(selectedTime);
    }
  });
  document.addEventListener('pointerdown', event => {
    if (!(event.target instanceof Node)) return;
    for (const chart of document.querySelectorAll('[data-chart]')) {
      if (!chart.querySelector('svg')?.contains(event.target)) chartTooltips.get(chart)?.();
    }
  });
  document.addEventListener('DOMContentLoaded', () => initialize(document));
  document.addEventListener('htmx:afterSwap', event => initialize(event.target));
})();
