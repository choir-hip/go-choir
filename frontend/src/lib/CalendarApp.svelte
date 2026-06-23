<script>
  import { createEventDispatcher, onMount, tick } from 'svelte';
  import {
    loadRecentMedia,
    mediaSourceIdentity,
    recentMediaAppContext,
    rememberRecentMedia,
    resolveMediaSource,
  } from './media-utils.js';
  import { addLiveEventListener, liveEventKind, liveEventPayload } from './live-events.js';

  export let appContext = {};
  export let windowId = '';
  export let authenticated = false;

  const kind = 'calendar';
  const dispatch = createEventDispatcher();

  const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
  const MONTHS = [
    'January', 'February', 'March', 'April', 'May', 'June',
    'July', 'August', 'September', 'October', 'November', 'December',
  ];

  let viewMode = 'month';
  let viewDate = new Date();
  viewDate.setDate(1);
  let events = [];
  let selectedEvent = null;
  let loading = false;
  let error = '';
  let item = appContext?.contentItem || null;
  let selectedContext = null;
  let recentFiles = [];
  let rememberedIdentity = '';
  let loadedSourceKey = '';
  let dragOver = false;

  $: effectiveContext = selectedContext || appContext || {};
  $: source = resolveMediaSource(effectiveContext, item, kind);
  $: sourceKey = source.displayUrl || '';
  $: sourceIdentity = mediaSourceIdentity(source);
  $: if (source.displayUrl && sourceIdentity && sourceIdentity !== rememberedIdentity) {
    void rememberCurrentSource();
  }

  $: monthLabel = `${MONTHS[viewDate.getMonth()]} ${viewDate.getFullYear()}`;
  $: calendarDays = buildCalendarDays(viewDate, events);
  $: agendaEvents = buildAgendaEvents(viewDate, events);

  async function refreshRecentFiles() {
    recentFiles = await loadRecentMedia(kind);
  }

  async function rememberCurrentSource() {
    rememberedIdentity = sourceIdentity;
    if (await rememberRecentMedia(kind, source)) {
      await refreshRecentFiles();
    }
  }

  function sourceFetchOptions(url) {
    return String(url || '').startsWith('/') ? { credentials: 'include' } : { credentials: 'omit' };
  }

  // ---- ICS Parser ----
  function parseIcs(text) {
    const lines = unfoldIcsLines(text.split(/\r?\n/));
    const parsed = [];
    let current = null;
    let inAlarm = false;

    for (const line of lines) {
      if (line === 'BEGIN:VEVENT') {
        current = { props: {} };
        inAlarm = false;
      } else if (line === 'BEGIN:VALARM') {
        inAlarm = true;
      } else if (line === 'END:VALARM') {
        inAlarm = false;
      } else if (line === 'END:VEVENT') {
        if (current) {
          const event = buildEventFromProps(current.props);
          if (event) parsed.push(event);
        }
        current = null;
      } else if (current && !inAlarm) {
        const parsed_line = parseIcsLine(line);
        if (parsed_line) {
          current.props[parsed_line.name] = parsed_line;
        }
      }
    }
    return parsed;
  }

  function unfoldIcsLines(lines) {
    const result = [];
    for (const line of lines) {
      if ((line.startsWith(' ') || line.startsWith('\t')) && result.length > 0) {
        result[result.length - 1] += line.slice(1);
      } else {
        result.push(line);
      }
    }
    return result;
  }

  function parseIcsLine(line) {
    const colonIdx = line.indexOf(':');
    if (colonIdx < 0) return null;
    const fullKey = line.slice(0, colonIdx);
    const value = line.slice(colonIdx + 1);
    const keyParts = fullKey.split(';');
    const name = keyParts[0].toUpperCase();
    const params = {};
    for (let i = 1; i < keyParts.length; i++) {
      const eqIdx = keyParts[i].indexOf('=');
      if (eqIdx > 0) {
        params[keyParts[i].slice(0, eqIdx).toUpperCase()] = keyParts[i].slice(eqIdx + 1);
      }
    }
    return { name, value, params };
  }

  function unescapeIcsText(value) {
    return String(value || '')
      .replace(/\\n/gi, '\n')
      .replace(/\\,/g, ',')
      .replace(/\\;/g, ';')
      .replace(/\\\\/g, '\\');
  }

  function parseIcsDate(value, params) {
    if (!value) return null;
    if (params.VALUE === 'DATE' || /^\d{8}$/.test(value)) {
      const y = parseInt(value.slice(0, 4), 10);
      const m = parseInt(value.slice(4, 6), 10) - 1;
      const d = parseInt(value.slice(6, 8), 10);
      return new Date(y, m, d);
    }
    if (/^\d{8}T\d{6}Z$/.test(value)) {
      const y = parseInt(value.slice(0, 4), 10);
      const m = parseInt(value.slice(4, 6), 10) - 1;
      const d = parseInt(value.slice(6, 8), 10);
      const h = parseInt(value.slice(9, 11), 10);
      const mi = parseInt(value.slice(11, 13), 10);
      const s = parseInt(value.slice(13, 15), 10);
      return new Date(Date.UTC(y, m, d, h, mi, s));
    }
    if (/^\d{8}T\d{6}$/.test(value)) {
      const y = parseInt(value.slice(0, 4), 10);
      const m = parseInt(value.slice(4, 6), 10) - 1;
      const d = parseInt(value.slice(6, 8), 10);
      const h = parseInt(value.slice(9, 11), 10);
      const mi = parseInt(value.slice(11, 13), 10);
      const s = parseInt(value.slice(13, 15), 10);
      return new Date(y, m, d, h, mi, s);
    }
    return null;
  }

  function buildEventFromProps(props) {
    const summary = props.SUMMARY ? unescapeIcsText(props.SUMMARY.value) : 'Untitled Event';
    const dtStart = parseIcsDate(props.DTSTART?.value, props.DTSTART?.params || {});
    const dtEnd = parseIcsDate(props.DTEND?.value, props.DTEND?.params || {});
    if (!dtStart) return null;
    const allDay = (props.DTSTART?.params?.VALUE === 'DATE') || /^\d{8}$/.test(props.DTSTART?.value || '');
    const location = props.LOCATION ? unescapeIcsText(props.LOCATION.value) : '';
    const description = props.DESCRIPTION ? unescapeIcsText(props.DESCRIPTION.value) : '';
    const organizer = props.ORGANIZER?.value || '';
    const attendees = [];
    for (const key of Object.keys(props)) {
      if (key === 'ATTENDEE') {
        const val = props[key].value;
        if (val) attendees.push(val.replace(/^MAILTO:/i, ''));
      }
    }
    const rrule = props.RRULE?.value || '';

    const event = {
      id: `${summary}-${dtStart.getTime()}`,
      title: summary,
      start: dtStart,
      end: dtEnd,
      allDay,
      location,
      description,
      organizer: organizer.replace(/^MAILTO:/i, ''),
      attendees,
      rrule,
    };

    if (rrule) {
      return expandRecurring(event);
    }
    return [event];
  }

  function expandRecurring(event, maxCount = 100) {
    const result = [event];
    const rule = parseRrule(event.rrule);
    if (!rule.freq) return result;

    let current = new Date(event.start);
    let count = 1;
    const until = rule.until ? new Date(rule.until) : new Date(event.start.getFullYear() + 2, 11, 31);
    const interval = rule.interval || 1;

    while (count < maxCount && current < until) {
      if (rule.count && count >= rule.count) break;
      current = nextOccurrence(current, rule.freq, interval);
      if (current > until) break;
      const end = event.end ? new Date(current.getTime() + (event.end - event.start)) : null;
      result.push({
        ...event,
        id: `${event.id}-${count}`,
        start: new Date(current),
        end,
      });
      count++;
    }
    return result;
  }

  function parseRrule(rrule) {
    const parts = {};
    for (const part of rrule.split(';')) {
      const [key, value] = part.split('=');
      parts[key.toUpperCase()] = value;
    }
    return {
      freq: parts.FREQ?.toUpperCase(),
      interval: parseInt(parts.INTERVAL || '1', 10),
      count: parseInt(parts.COUNT || '0', 10),
      until: parts.UNTIL ? parseIcsDate(parts.UNTIL, {}) : null,
    };
  }

  function nextOccurrence(date, freq, interval) {
    const next = new Date(date);
    switch (freq) {
      case 'DAILY':
        next.setDate(next.getDate() + interval);
        break;
      case 'WEEKLY':
        next.setDate(next.getDate() + 7 * interval);
        break;
      case 'MONTHLY':
        next.setMonth(next.getMonth() + interval);
        break;
      case 'YEARLY':
        next.setFullYear(next.getFullYear() + interval);
        break;
      default:
        next.setDate(next.getDate() + 1);
    }
    return next;
  }

  // ---- Calendar grid ----
  function buildCalendarDays(monthDate, allEvents) {
    const year = monthDate.getFullYear();
    const month = monthDate.getMonth();
    const firstDay = new Date(year, month, 1);
    const startWeekday = firstDay.getDay();
    const daysInMonth = new Date(year, month + 1, 0).getDate();

    const days = [];
    // Leading days from previous month
    for (let i = 0; i < startWeekday; i++) {
      const d = new Date(year, month, -startWeekday + i + 1);
      days.push({ date: d, inMonth: false, events: eventsOnDay(allEvents, d) });
    }
    // Current month days
    for (let d = 1; d <= daysInMonth; d++) {
      const date = new Date(year, month, d);
      days.push({ date, inMonth: true, events: eventsOnDay(allEvents, date) });
    }
    // Trailing days to fill the grid (42 cells = 6 weeks)
    while (days.length < 42) {
      const lastDate = days[days.length - 1].date;
      const d = new Date(lastDate);
      d.setDate(d.getDate() + 1);
      days.push({ date: d, inMonth: false, events: eventsOnDay(allEvents, d) });
    }
    return days;
  }

  function eventsOnDay(allEvents, date) {
    return allEvents.filter((event) => {
      const eventDate = new Date(event.start);
      return eventDate.getFullYear() === date.getFullYear() &&
        eventDate.getMonth() === date.getMonth() &&
        eventDate.getDate() === date.getDate();
    });
  }

  function buildAgendaEvents(monthDate, allEvents) {
    const year = monthDate.getFullYear();
    const month = monthDate.getMonth();
    const monthStart = new Date(year, month, 1);
    const monthEnd = new Date(year, month + 1, 0, 23, 59, 59);
    return allEvents
      .filter((event) => event.start >= monthStart && event.start <= monthEnd)
      .sort((a, b) => a.start - b.start);
  }

  // ---- Navigation ----
  function prevMonth() {
    viewDate = new Date(viewDate.getFullYear(), viewDate.getMonth() - 1, 1);
  }

  function nextMonth() {
    viewDate = new Date(viewDate.getFullYear(), viewDate.getMonth() + 1, 1);
  }

  function goToday() {
    viewDate = new Date();
    viewDate.setDate(1);
  }

  function isToday(date) {
    const now = new Date();
    return date.getFullYear() === now.getFullYear() &&
      date.getMonth() === now.getMonth() &&
      date.getDate() === now.getDate();
  }

  function formatEventTime(event) {
    if (event.allDay) return 'All day';
    const time = event.start.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
    if (event.end) {
      const endTime = event.end.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
      return `${time} – ${endTime}`;
    }
    return time;
  }

  function formatDate(date) {
    return date.toLocaleDateString([], { weekday: 'long', month: 'long', day: 'numeric', year: 'numeric' });
  }

  // ---- File loading ----
  async function loadIcs() {
    if (!sourceKey || loadedSourceKey === sourceKey) return;
    loadedSourceKey = sourceKey;
    loading = true;
    error = '';
    try {
      const res = await fetch(sourceKey, sourceFetchOptions(sourceKey));
      if (!res.ok) throw new Error(`Fetch failed (${res.status})`);
      const text = await res.text();
      const parsed = parseIcs(text);
      events = [...events, ...parsed];
    } catch (err) {
      error = err?.message || 'Failed to load calendar file';
    } finally {
      loading = false;
    }
  }

  async function loadContentItem() {
    loading = true;
    error = '';
    const { loadContextContentItem } = await import('./media-utils.js');
    const result = await loadContextContentItem(effectiveContext, item, 'Calendar');
    loading = false;
    if (result.authRequired) {
      dispatch('authexpired');
      return;
    }
    if (result.error) {
      error = result.error;
      return;
    }
    if (result.item) item = result.item;
    await tick();
    await loadIcs();
  }

  async function openRecentFile(entry) {
    selectedContext = recentMediaAppContext(entry);
    item = null;
    error = '';
    loadedSourceKey = '';
    dispatch('contextchange', { windowId, appContext: selectedContext, title: selectedContext.windowTitle });
    await tick();
    await loadContentItem();
  }

  // ---- Drag & drop ----
  function handleDragOver(event) {
    event.preventDefault();
    dragOver = true;
  }

  function handleDragLeave() {
    dragOver = false;
  }

  async function handleDrop(event) {
    event.preventDefault();
    dragOver = false;
    const files = event.dataTransfer?.files;
    if (!files || files.length === 0) return;
    for (const file of files) {
      if (file.name.toLowerCase().endsWith('.ics')) {
        const text = await file.text();
        const parsed = parseIcs(text);
        events = [...events, ...parsed];
      }
    }
  }

  // ---- Import from file input ----
  async function handleFileInput(event) {
    const files = event.target?.files;
    if (!files) return;
    for (const file of files) {
      if (file.name.toLowerCase().endsWith('.ics')) {
        const text = await file.text();
        const parsed = parseIcs(text);
        events = [...events, ...parsed];
      }
    }
    event.target.value = '';
  }

  function clearEvents() {
    events = [];
    selectedEvent = null;
    loadedSourceKey = '';
  }

  onMount(() => {
    void refreshRecentFiles();
    if (sourceKey) void loadContentItem();
    const removeLiveListener = addLiveEventListener((message) => {
      if (liveEventKind(message) === 'media.recent.updated' && liveEventPayload(message).kind === kind) {
        void refreshRecentFiles();
      }
    });
    return () => {
      removeLiveListener();
    };
  });
</script>

<section
  class="calendar-app"
  data-calendar-app
  on:dragover={handleDragOver}
  on:dragleave={handleDragLeave}
  on:drop={handleDrop}
  class:drag-over={dragOver}
>
  <header class="cal-header">
    <div class="cal-nav">
      <button class="cal-btn" on:click={prevMonth} title="Previous month">‹</button>
      <h2 class="cal-month-label">{monthLabel}</h2>
      <button class="cal-btn" on:click={nextMonth} title="Next month">›</button>
    </div>
    <div class="cal-actions">
      <button class="cal-btn cal-btn-today" on:click={goToday}>Today</button>
      <div class="cal-view-toggle">
        <button class:active={viewMode === 'month'} on:click={() => (viewMode = 'month')}>Month</button>
        <button class:active={viewMode === 'agenda'} on:click={() => (viewMode = 'agenda')}>Agenda</button>
      </div>
      <label class="cal-import-btn">
        <span>Import .ics</span>
        <input type="file" accept=".ics,text/calendar" on:change={handleFileInput} hidden />
      </label>
      {#if events.length > 0}
        <button class="cal-btn cal-btn-clear" on:click={clearEvents} title="Clear all events">Clear</button>
      {/if}
    </div>
  </header>

  {#if error}
    <div class="cal-error" role="alert">{error}</div>
  {/if}

  {#if loading}
    <div class="cal-loading">Loading calendar…</div>
  {:else if viewMode === 'month'}
    <div class="cal-month-view">
      <div class="cal-weekday-row">
        {#each WEEKDAYS as day}
          <div class="cal-weekday">{day}</div>
        {/each}
      </div>
      <div class="cal-day-grid">
        {#each calendarDays as day}
          <div
            class="cal-day {day.inMonth ? 'in-month' : 'out-of-month'} {isToday(day.date) ? 'is-today' : ''}"
            on:click={() => {
              if (day.events.length === 1) {
                selectedEvent = day.events[0];
              } else if (day.events.length > 1) {
                viewDate = new Date(day.date.getFullYear(), day.date.getMonth(), 1);
                viewMode = 'agenda';
              }
            }}
          >
            <span class="cal-day-num">{day.date.getDate()}</span>
            {#if day.events.length > 0}
              <div class="cal-day-events">
                {#each day.events.slice(0, 3) as event}
                  <div class="cal-day-event" title={event.title}>
                    <span class="cal-event-dot"></span>
                    <span class="cal-event-label">{event.title}</span>
                  </div>
                {/each}
                {#if day.events.length > 3}
                  <div class="cal-day-more">+{day.events.length - 3} more</div>
                {/if}
              </div>
            {/if}
          </div>
        {/each}
      </div>
    </div>
  {:else}
    <div class="cal-agenda-view">
      {#if agendaEvents.length === 0}
        <div class="cal-empty">No events this month</div>
      {:else}
        {#each agendaEvents as event}
          <button class="cal-agenda-item" on:click={() => (selectedEvent = event)}>
            <div class="cal-agenda-date">
              <span class="cal-agenda-day">{event.start.getDate()}</span>
              <span class="cal-agenda-month">{MONTHS[event.start.getMonth()].slice(0, 3)}</span>
            </div>
            <div class="cal-agenda-info">
              <span class="cal-agenda-title">{event.title}</span>
              <span class="cal-agenda-time">{formatEventTime(event)}</span>
              {#if event.location}
                <span class="cal-agenda-location">📍 {event.location}</span>
              {/if}
            </div>
          </button>
        {/each}
      {/if}
    </div>
  {/if}

  {#if events.length === 0 && !loading && !error}
    <div class="cal-empty-state">
      <div class="cal-empty-icon">📅</div>
      <p>No events loaded. Import an <code>.ics</code> file from Files or drag one here.</p>
      {#if recentFiles.length > 0}
        <div class="cal-recent">
          <h3>Recent</h3>
          {#each recentFiles as entry}
            <button class="cal-recent-item" on:click={() => openRecentFile(entry)}>
              <span class="cal-recent-icon">📅</span>
              <span class="cal-recent-name">{entry.title || entry.fileName || 'Untitled'}</span>
            </button>
          {/each}
        </div>
      {/if}
    </div>
  {/if}

  <!-- Event detail panel -->
  {#if selectedEvent}
    <div class="cal-event-overlay" on:click={() => (selectedEvent = null)} on:keydown={(e) => e.key === 'Escape' && (selectedEvent = null)} tabindex="-1">
      <div class="cal-event-detail" on:click|stopPropagation role="dialog" aria-label="Event details">
        <button class="cal-event-close" on:click={() => (selectedEvent = null)}>×</button>
        <h3>{selectedEvent.title}</h3>
        <div class="cal-event-row">
          <span class="cal-event-icon">🕐</span>
          <div>
            <strong>{formatDate(selectedEvent.start)}</strong>
            <p>{formatEventTime(selectedEvent)}</p>
          </div>
        </div>
        {#if selectedEvent.location}
          <div class="cal-event-row">
            <span class="cal-event-icon">📍</span>
            <div><strong>Location</strong><p>{selectedEvent.location}</p></div>
          </div>
        {/if}
        {#if selectedEvent.description}
          <div class="cal-event-row">
            <span class="cal-event-icon">📝</span>
            <div>
              <strong>Description</strong>
              <p class="cal-event-desc">{selectedEvent.description}</p>
            </div>
          </div>
        {/if}
        {#if selectedEvent.organizer}
          <div class="cal-event-row">
            <span class="cal-event-icon">👤</span>
            <div><strong>Organizer</strong><p>{selectedEvent.organizer}</p></div>
          </div>
        {/if}
        {#if selectedEvent.attendees.length > 0}
          <div class="cal-event-row">
            <span class="cal-event-icon">👥</span>
            <div>
              <strong>Attendees</strong>
              <ul class="cal-attendees">
                {#each selectedEvent.attendees as attendee}
                  <li>{attendee}</li>
                {/each}
              </ul>
            </div>
          </div>
        {/if}
      </div>
    </div>
  {/if}
</section>

<style>
  .calendar-app {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
    background: var(--choir-surface-app, #1a1a2e);
    color: var(--choir-text-primary, #e0e0e0);
  }

  .calendar-app.drag-over {
    outline: 2px dashed var(--choir-border-strong, #555);
    outline-offset: -8px;
  }

  .cal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    padding: 0.75rem 1.25rem;
    border-bottom: 1px solid var(--choir-border-strong, rgba(255,255,255,0.08));
    flex-wrap: wrap;
  }

  .cal-nav {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .cal-month-label {
    font-size: 1.25rem;
    font-weight: 700;
    margin: 0;
    min-width: 180px;
    text-align: center;
  }

  .cal-actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .cal-btn {
    display: grid;
    place-items: center;
    min-width: 32px;
    height: 32px;
    padding: 0 0.6rem;
    border: 1px solid var(--choir-border-strong, rgba(255,255,255,0.1));
    border-radius: 6px;
    background: rgba(255,255,255,0.05);
    color: var(--choir-text-primary, #e0e0e0);
    font: inherit;
    font-size: 0.85rem;
    cursor: pointer;
    transition: background 0.15s;
  }

  .cal-btn:hover {
    background: rgba(255,255,255,0.1);
  }

  .cal-btn-today {
    font-weight: 600;
  }

  .cal-btn-clear {
    color: var(--choir-danger, #ff6b6b);
    border-color: rgba(255,107,107,0.3);
  }

  .cal-view-toggle {
    display: flex;
    border: 1px solid var(--choir-border-strong, rgba(255,255,255,0.1));
    border-radius: 6px;
    overflow: hidden;
  }

  .cal-view-toggle button {
    padding: 0.4rem 0.75rem;
    border: none;
    background: transparent;
    color: var(--choir-text-primary, #e0e0e0);
    font: inherit;
    font-size: 0.85rem;
    cursor: pointer;
    transition: background 0.15s;
  }

  .cal-view-toggle button.active {
    background: rgba(255,255,255,0.1);
    font-weight: 600;
  }

  .cal-import-btn {
    display: grid;
    place-items: center;
    height: 32px;
    padding: 0 0.75rem;
    border: 1px solid var(--choir-border-strong, rgba(255,255,255,0.1));
    border-radius: 6px;
    background: rgba(124,158,255,0.15);
    color: #7c9eff;
    font: inherit;
    font-size: 0.85rem;
    font-weight: 600;
    cursor: pointer;
    transition: background 0.15s;
  }

  .cal-import-btn:hover {
    background: rgba(124,158,255,0.25);
  }

  .cal-error {
    padding: 0.75rem 1.25rem;
    color: var(--choir-danger, #ff6b6b);
    font-size: 0.9rem;
  }

  .cal-loading {
    padding: 2rem;
    text-align: center;
    color: var(--choir-muted, #888);
  }

  /* Month view */
  .cal-month-view {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    padding: 0.5rem;
  }

  .cal-weekday-row {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    gap: 2px;
    margin-bottom: 2px;
  }

  .cal-weekday {
    text-align: center;
    font-size: 0.7rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--choir-muted, #888);
    padding: 0.4rem 0;
  }

  .cal-day-grid {
    flex: 1;
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    grid-template-rows: repeat(6, 1fr);
    gap: 2px;
  }

  .cal-day {
    display: flex;
    flex-direction: column;
    padding: 0.3rem;
    border-radius: 4px;
    background: rgba(255,255,255,0.02);
    cursor: pointer;
    overflow: hidden;
    transition: background 0.15s;
    min-height: 0;
  }

  .cal-day:hover {
    background: rgba(255,255,255,0.06);
  }

  .cal-day.out-of-month {
    opacity: 0.35;
  }

  .cal-day.is-today {
    background: rgba(124,158,255,0.12);
    border: 1px solid rgba(124,158,255,0.3);
  }

  .cal-day-num {
    font-size: 0.8rem;
    font-weight: 600;
    margin-bottom: 0.15rem;
  }

  .cal-day.is-today .cal-day-num {
    color: #7c9eff;
  }

  .cal-day-events {
    display: flex;
    flex-direction: column;
    gap: 1px;
    overflow: hidden;
  }

  .cal-day-event {
    display: flex;
    align-items: center;
    gap: 3px;
    font-size: 0.65rem;
    overflow: hidden;
  }

  .cal-event-dot {
    flex: none;
    width: 5px;
    height: 5px;
    border-radius: 50%;
    background: #7c9eff;
  }

  .cal-event-label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .cal-day-more {
    font-size: 0.6rem;
    color: var(--choir-muted, #888);
    padding-left: 8px;
  }

  /* Agenda view */
  .cal-agenda-view {
    flex: 1;
    overflow: auto;
    padding: 0.5rem 1rem;
  }

  .cal-agenda-item {
    display: flex;
    align-items: flex-start;
    gap: 1rem;
    width: 100%;
    padding: 0.75rem 1rem;
    margin-bottom: 0.4rem;
    border: 1px solid var(--choir-border-strong, rgba(255,255,255,0.08));
    border-radius: 8px;
    background: rgba(255,255,255,0.03);
    color: inherit;
    text-align: left;
    cursor: pointer;
    transition: background 0.15s;
  }

  .cal-agenda-item:hover {
    background: rgba(255,255,255,0.07);
  }

  .cal-agenda-date {
    display: flex;
    flex-direction: column;
    align-items: center;
    flex: none;
    width: 48px;
  }

  .cal-agenda-day {
    font-size: 1.5rem;
    font-weight: 700;
    line-height: 1;
  }

  .cal-agenda-month {
    font-size: 0.7rem;
    text-transform: uppercase;
    color: var(--choir-muted, #888);
  }

  .cal-agenda-info {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    min-width: 0;
  }

  .cal-agenda-title {
    font-weight: 600;
    font-size: 0.95rem;
  }

  .cal-agenda-time {
    font-size: 0.8rem;
    color: var(--choir-muted, #aaa);
  }

  .cal-agenda-location {
    font-size: 0.8rem;
    color: var(--choir-muted, #888);
  }

  .cal-empty {
    padding: 2rem;
    text-align: center;
    color: var(--choir-muted, #888);
  }

  /* Empty state */
  .cal-empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 1rem;
    padding: 2rem;
    text-align: center;
    flex: 1;
  }

  .cal-empty-icon {
    font-size: 3rem;
    opacity: 0.6;
  }

  .cal-empty-state > p {
    color: var(--choir-muted, #888);
    max-width: 320px;
    margin: 0;
  }

  .cal-empty-state code {
    background: rgba(255,255,255,0.08);
    padding: 0.1rem 0.3rem;
    border-radius: 3px;
    font-size: 0.85rem;
  }

  .cal-recent {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    max-width: 360px;
    width: 100%;
    text-align: left;
  }

  .cal-recent h3 {
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--choir-muted, #666);
    margin: 0;
  }

  .cal-recent-item {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 0.75rem;
    background: rgba(255,255,255,0.05);
    border: 1px solid rgba(255,255,255,0.08);
    border-radius: 6px;
    color: #ccc;
    cursor: pointer;
    text-align: left;
    transition: background 0.15s;
  }

  .cal-recent-item:hover {
    background: rgba(255,255,255,0.1);
  }

  .cal-recent-icon {
    font-size: 1rem;
  }

  .cal-recent-name {
    font-size: 0.85rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* Event detail overlay */
  .cal-event-overlay {
    position: absolute;
    inset: 0;
    background: rgba(0,0,0,0.5);
    display: grid;
    place-items: center;
    z-index: 100;
    padding: 1rem;
  }

  .cal-event-detail {
    position: relative;
    max-width: 440px;
    width: 100%;
    max-height: 80%;
    overflow: auto;
    padding: 1.5rem;
    background: var(--choir-surface-app, #252538);
    border: 1px solid var(--choir-border-strong, rgba(255,255,255,0.12));
    border-radius: 12px;
    box-shadow: 0 8px 32px rgba(0,0,0,0.4);
  }

  .cal-event-close {
    position: absolute;
    top: 0.75rem;
    right: 0.75rem;
    width: 28px;
    height: 28px;
    display: grid;
    place-items: center;
    border: none;
    border-radius: 50%;
    background: rgba(255,255,255,0.08);
    color: var(--choir-text-primary, #e0e0e0);
    font-size: 1.1rem;
    cursor: pointer;
  }

  .cal-event-close:hover {
    background: rgba(255,255,255,0.15);
  }

  .cal-event-detail h3 {
    font-size: 1.25rem;
    font-weight: 700;
    margin: 0 0 1rem;
    padding-right: 2rem;
  }

  .cal-event-row {
    display: flex;
    gap: 0.75rem;
    margin-bottom: 0.85rem;
  }

  .cal-event-icon {
    flex: none;
    font-size: 1.1rem;
    margin-top: 0.1rem;
  }

  .cal-event-row strong {
    font-size: 0.85rem;
    display: block;
    margin-bottom: 0.15rem;
  }

  .cal-event-row p {
    margin: 0;
    font-size: 0.9rem;
    color: var(--choir-muted, #aaa);
    line-height: 1.4;
  }

  .cal-event-desc {
    white-space: pre-wrap;
  }

  .cal-attendees {
    margin: 0;
    padding-left: 1.1rem;
    font-size: 0.85rem;
    color: var(--choir-muted, #aaa);
  }

  .cal-attendees li {
    margin-bottom: 0.15rem;
  }

  @media (max-width: 760px) {
    .cal-header {
      padding: 0.5rem 0.75rem;
    }

    .cal-month-label {
      font-size: 1.1rem;
      min-width: 140px;
    }

    .cal-actions {
      gap: 0.3rem;
    }

    .cal-day-event {
      font-size: 0.55rem;
    }

    .cal-event-label {
      display: none;
    }

    .cal-day-events .cal-day-event {
      gap: 0;
    }

    .cal-event-dot {
      width: 6px;
      height: 6px;
    }
  }
</style>
