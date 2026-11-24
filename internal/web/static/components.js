(() => {
  const svgNS = 'http://www.w3.org/2000/svg';
  const colors = new Set(['blue', 'amber', 'purple', 'green', 'red']);
  const columnStates = new WeakMap();
  const chartSelections = new WeakMap();
  const chartTooltips = new WeakMap();
  const timelineSelections = new WeakMap();
  const timelineLayouts = new WeakMap();

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
    if (timeline.dataset.timelineReady === 'true') {
      timelineLayouts.get(timeline)?.();
      return;
    }
    const slider = timeline.querySelector('[data-timeline-slider]');
    const output = timeline.querySelector('[data-timeline-output]');
    const viewport = timeline.querySelector('[data-timeline-viewport]');
    const axis = timeline.querySelector('[data-timeline-axis]');
    const track = timeline.querySelector('.timeline-track');
    const zoom = timeline.querySelector('[data-timeline-zoom]');
    const tooltip = timeline.querySelector('[data-timeline-tooltip]');
    const start = Number(timeline.dataset.start);
    const end = Number(timeline.dataset.end);
    if (!slider || !output || !viewport || !axis || !track || !zoom || !tooltip ||
        !Number.isFinite(start) || !Number.isFinite(end) || start >= end) return;
    const step = Number(slider.step) || 1000;
    const format = time => new Date(time).toLocaleString(undefined, {
      month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit', second: '2-digit', timeZoneName: 'short',
      fractionalSecondDigits: time % 1000 ? 3 : undefined,
    });
    const clock = time => new Date(time).toLocaleTimeString(undefined, {
      hour: '2-digit', minute: '2-digit', second: '2-digit',
    });
    let selected = end;
    const scale = () => (end - start) / 2 ** Number(zoom.value) / Math.max(1, viewport.clientHeight);
    const anchor = () => Math.min(72, viewport.clientHeight * 0.12);
    const pixel = time => anchor() + (selected - time) / scale();
    const hideTooltip = () => { tooltip.hidden = true; };
    const render = () => {
      hideTooltip();
      zoom.setAttribute('aria-valuetext', `${Math.round((end - start) / 2 ** Number(zoom.value) / 1000)} seconds across the viewport; up to one hour retained`);
      axis.replaceChildren();
      const height = viewport.clientHeight;
      const msPerPixel = scale();
      const topTime = Math.min(end, selected + anchor() * msPerPixel);
      const bottomTime = Math.max(start, selected - (height - anchor()) * msPerPixel);
      const intervals = [1000, 5000, 10000, 15000, 30000, 60000, 120000, 300000, 600000, 900000];
      const major = intervals.find(interval => interval / msPerPixel >= 110) ?? 900000;
      const minor = major / 10;
      for (let time = Math.ceil(bottomTime / minor) * minor; time <= topTime; time += minor) {
        const tick = document.createElement('span');
        tick.className = 'timeline-tick';
        tick.style.top = `${pixel(time)}px`;
        if (time % major === 0) {
          tick.classList.add('timeline-tick-major');
          tick.textContent = new Date(time).toLocaleTimeString(undefined, {
            hour: '2-digit', minute: '2-digit', ...(major < 60000 ? { second: '2-digit' } : {}),
          });
        }
        axis.append(tick);
      }
      for (const span of timeline.querySelectorAll('[data-timeline-span]')) {
        const from = Number(span.dataset.spanStart);
        const length = Number(span.dataset.spanHeight);
        const valid = Number.isFinite(from) && Number.isFinite(length) &&
          from >= 0 && length > 0 && from + length <= 100.001;
        const spanStart = start + (end - start) * from / 100;
        const spanEnd = Math.min(end, spanStart + (end - start) * length / 100);
        const top = Math.max(0, pixel(spanEnd));
        const bottom = Math.min(height, pixel(spanStart));
        span.hidden = !valid || bottom <= top;
        span.style.top = `${top}px`;
        span.style.height = `${Math.max(0, bottom - top)}px`;
        span.tabIndex = span.hidden ? -1 : 0;
        span.removeAttribute('title');
        span.dataset.timelineLabel ??= span.getAttribute('aria-label');
        span.dataset.timelineDescription = `${span.dataset.timelineLabel} · ${format(spanStart)} – ${format(spanEnd)}`;
        span.setAttribute('aria-label', span.dataset.timelineDescription);
      }
      for (const group of track.querySelectorAll('[data-timeline-group]')) group.remove();
      const groups = [];
      const markers = [...track.querySelectorAll('[data-timeline-marker]')].sort((a, b) =>
        Number(b.dataset.timelineSelect) - Number(a.dataset.timelineSelect));
      for (const marker of markers) {
        const y = pixel(Number(marker.dataset.timelineSelect));
        marker.hidden = y < 12 || y > height - 12;
        marker.style.top = `${y}px`;
        if (marker.hidden) continue;
        const group = groups.at(-1);
        // Adjacent badges need 28px. Connected overlaps form one cluster;
        // zooming increases their separation until individual badges fit.
        if (group && y - group.last < 28) {
          group.markers.push(marker);
          group.last = y;
        } else groups.push({ markers: [marker], first: y, last: y });
      }
      for (const group of groups) {
        if (group.markers.length === 1) continue;
        const button = document.createElement('button');
        button.type = 'button';
        button.className = 'timeline-marker timeline-cluster';
        button.dataset.timelineGroup = '';
        const times = group.markers.map(marker => Number(marker.dataset.timelineSelect));
        button.dataset.timelineSelect = String(Math.round((times[0] + times.at(-1)) / 2));
        const zoomHint = Number(zoom.value) < Number(zoom.max)
          ? 'Select to zoom in.' : 'Maximum timeline zoom; use the observed event records below for details.';
        button.dataset.timelineDescription = `${times.length} records · ${clock(times.at(-1))} – ${clock(times[0])}. ${zoomHint}`;
        button.setAttribute('aria-label', button.dataset.timelineDescription);
        button.style.top = `${(group.first + group.last) / 2}px`;
        for (const marker of group.markers) {
          marker.hidden = true;
          const dot = document.createElement('span');
          dot.className = 'timeline-dot';
          dot.style.top = `${parseFloat(marker.style.top) - (group.first + group.last) / 2 + 12}px`;
          dot.setAttribute('aria-hidden', 'true');
          button.append(dot);
        }
        const badge = document.createElement('span');
        badge.className = 'timeline-badge';
        badge.textContent = String(times.length);
        badge.setAttribute('aria-hidden', 'true');
        button.append(badge);
        track.append(button);
      }
    };
    const setTime = time => {
      selected = Math.round(Math.max(start, Math.min(end, time)));
      const sliderTime = Math.max(start, Math.min(end,
        start + Math.round((selected - start) / step) * step));
      slider.value = String(sliderTime);
      timeline.dataset.selected = String(selected);
      const label = format(selected);
      output.value = selected === end ? 'Latest' : clock(selected);
      output.textContent = output.value;
      output.title = label;
      slider.setAttribute('aria-valuetext', label);
      render();
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
    const select = time => {
      if (!Number.isFinite(time)) return;
      timeline.dispatchEvent(new CustomEvent('observability:timechange', {
        bubbles: true, detail: { time: Math.round(Math.max(start, Math.min(end, time))) },
      }));
    };
    slider.addEventListener('input', () => select(Number(slider.value)));
    slider.addEventListener('keydown', event => {
      if (!['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'Home', 'End', 'PageUp', 'PageDown'].includes(event.key)) return;
      event.preventDefault();
      if (event.key === 'Home') select(start);
      else if (event.key === 'End') select(end);
      else {
        const page = event.key.startsWith('Page');
        const increment = page ? (end - start) / 2 ** Number(zoom.value) / 5 : step;
        select((page ? selected : Number(slider.value)) +
          (['ArrowUp', 'ArrowRight', 'PageUp'].includes(event.key) ? increment : -increment));
      }
    });
    zoom.addEventListener('input', render);
    viewport.addEventListener('wheel', event => {
      event.preventDefault();
      const delta = event.deltaY * (event.deltaMode === 1 ? 16 : event.deltaMode === 2 ? viewport.clientHeight : 1);
      select(selected - delta * scale());
    }, { passive: false });
    let drag;
    viewport.addEventListener('pointerdown', event => {
      if (event.button !== 0 || event.target.closest('button, [data-timeline-span]')) return;
      drag = { y: event.clientY, time: selected, moved: false };
      viewport.setPointerCapture(event.pointerId);
      hideTooltip();
    });
    viewport.addEventListener('pointermove', event => {
      if (!drag) return;
      const delta = event.clientY - drag.y;
      drag.moved ||= Math.abs(delta) > 3;
      if (drag.moved) select(drag.time + delta * scale());
    });
    viewport.addEventListener('pointerup', event => {
      if (!drag) return;
      if (!drag.moved) select(selected + (anchor() - (event.clientY - viewport.getBoundingClientRect().top)) * scale());
      drag = undefined;
      viewport.releasePointerCapture(event.pointerId);
    });
    viewport.addEventListener('pointercancel', () => { drag = undefined; });
    const changeZoom = delta => {
      zoom.value = String(Math.max(Number(zoom.min), Math.min(Number(zoom.max), Number(zoom.value) + delta)));
      render();
    };
    timeline.addEventListener('click', event => {
      if (!(event.target instanceof Element)) return;
      if (event.target.closest('[data-timeline-zoom-in]')) return changeZoom(1);
      if (event.target.closest('[data-timeline-zoom-out]')) return changeZoom(-1);
      const button = event.target.closest('[data-timeline-select]');
      if (!button || !timeline.contains(button)) return;
      const time = Number(button.dataset.timelineSelect);
      if (!Number.isFinite(time)) return;
      const group = button.hasAttribute('data-timeline-group');
      if (group) changeZoom(1);
      select(time);
      if (group || button.hidden) slider.focus({ preventScroll: true });
    });
    const showTooltip = event => {
      const target = event.target.closest?.('[data-timeline-description]');
      if (!target || !timeline.contains(target)) return;
      tooltip.textContent = target.dataset.timelineDescription;
      tooltip.hidden = false;
      const bounds = target.getBoundingClientRect();
      const width = tooltip.getBoundingClientRect().width;
      tooltip.style.left = `${Math.max(8, Math.min(window.innerWidth - width - 8, bounds.right + 8))}px`;
      tooltip.style.top = `${Math.max(8, Math.min(window.innerHeight - tooltip.offsetHeight - 8, bounds.top))}px`;
    };
    timeline.addEventListener('pointerover', showTooltip);
    timeline.addEventListener('focusin', showTooltip);
    timeline.addEventListener('pointerout', hideTooltip);
    timeline.addEventListener('focusout', hideTooltip);
    timelineSelections.set(timeline, setTime);
    timelineLayouts.set(timeline, render);
    timeline.dataset.timelineReady = 'true';
    slider.disabled = false;
    timeline.querySelector('[data-timeline-zoom-controls]').hidden = false;
    timeline.querySelector('[data-timeline-cursor]').hidden = false;
    const latest = timeline.querySelector('[data-timeline-latest]');
    if (latest) latest.hidden = false;
    const initial = Number(timeline.dataset.selected);
    setTime(Number.isFinite(initial) ? initial : end);
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
  document.addEventListener('keydown', event => {
    if (event.key !== 'Escape') return;
    for (const tooltip of document.querySelectorAll('[data-timeline-tooltip]')) tooltip.hidden = true;
  });
  document.addEventListener('DOMContentLoaded', () => initialize(document));
  document.addEventListener('htmx:afterSwap', event => initialize(event.target));
  window.addEventListener('resize', () => {
    for (const timeline of document.querySelectorAll('[data-timeline]')) timelineLayouts.get(timeline)?.();
  });
})();
