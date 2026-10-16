export class UI {
  static #token = Symbol();
  static #instance;
  static #events = ['click', 'keydown', 'change', 'input', 'htmx:beforeRequest', 'htmx:afterSwap', 'htmx:afterRequest', 'htmx:sendAbort', 'htmx:sendError', 'htmx:timeout', 'htmx:responseError', 'htmx:swapError'];
  #environment;
  #controller;
  #refreshStatus;
  #integrationFilter;
  #filterTimer;
  #themePicker;
  #fontPicker;
  #sidebar;
  #sidebarToggle;
  #sidebarClose;
  #search;
  #searchRequest;
  #searchStatus;
  #searchDialog;
  #searchButton;
  #pageRoutes = [];
  #dismissTimers = new Map();
  #refreshSections = new Map();

  constructor(token) {
    const authorized = token === UI.#token;
    if (!authorized) throw new TypeError('Use UI.getInstance()');
  }

  static getInstance() {
    UI.#instance ??= new UI(UI.#token);
    const instance = UI.#instance;
    return instance;
  }

  init(environment = globalThis) {
    if (this.#controller) return this;
    const available = environment.document && environment.htmx;
    if (!available) return this;
    this.#environment = environment;
    this.#controller = new AbortController();
    const { document } = environment;
    this.#cacheElements();
    const { signal } = this.#controller;
    const options = { signal };
    UI.#events.forEach(name => document.addEventListener(name, this, options));
    this.#syncAppearance();
    this.#initNavigation();
    this.#initTimedFlashes();
    this.#initIntegrationFilter();
    this.#initToggleGroups();
    return this;
  }

  destroy() {
    clearTimeout(this.#filterTimer);
    this.#integrationFilter = undefined;
    this.dismissFlash(this.#refreshStatus, false);
    this.#refreshSections.forEach((sections, control) => this.#restoreSections(control));
    this.#dismissTimers.forEach(timer => clearTimeout(timer));
    this.#dismissTimers.clear();
    this.#searchRequest?.abort();
    this.#search?.unInit();
    this.#controller?.abort();
    this.#controller = undefined;
    this.#environment = undefined;
    this.#refreshStatus = undefined;
    this.#themePicker = undefined;
    this.#fontPicker = undefined;
    this.#sidebar = undefined;
    this.#sidebarToggle = undefined;
    this.#sidebarClose = undefined;
    this.#search = undefined;
    this.#searchRequest = undefined;
    this.#searchStatus = undefined;
    this.#searchDialog = undefined;
    this.#searchButton = undefined;
    this.#pageRoutes = [];
  }

  handleEvent(event) {
    this.#handleToggleGroup(event);
    if (event.type === 'input') {
      this.#scheduleIntegrationFilter(event);
      return;
    }
    if (event.type === 'htmx:afterSwap') {
      this.#initIntegrationFilter();
      this.#initToggleGroups();
    }
    this.#openSearch(event);
    this.#handleAppearancePickers(event);
    this.#handleSidebar(event);
    if (event.type === 'change') {
      this.#changeAppearance(event.target);
      return;
    }
    const clicked = event.type === 'click';
    if (clicked) {
      this.#dismissClickedFlash(event);
      return;
    }
    this.#updateRefresh(event);
    this.#reportProjectFilterError(event);
  }

  #reportProjectFilterError(event) {
    const errors = ['htmx:sendError', 'htmx:timeout', 'htmx:responseError', 'htmx:swapError'];
    if (!errors.includes(event.type)) return;
    const control = event.detail?.requestConfig?.elt ?? event.detail?.elt;
    if (!control?.matches?.('#project-query, #project-search, [data-project-page]')) return;
    this.#showRefreshToast('Could not filter projects. Try again.', false);
  }

  #cacheElements() {
    const { document, htmx } = this.#environment;
    const selectors = ['#page-refresh-status', '[data-theme-picker]', '#sidebar', '[data-sidebar-toggle]', '#search-status', '[data-sidebar-close]', '[data-font-picker]', '#search-dialog', '[data-search-open]'];
    const find = htmx.find.bind(htmx, document);
    const elements = selectors.map(find);
    [this.#refreshStatus, this.#themePicker, this.#sidebar, this.#sidebarToggle, this.#searchStatus, this.#sidebarClose, this.#fontPicker, this.#searchDialog, this.#searchButton] = elements;
  }

  #initToggleGroups() {
    const { document, htmx } = this.#environment;
    const buttons = htmx.findAll(document, '[data-toggle-group] [data-toggle-value]');
    const groups = new Set();
    buttons.forEach(button => {
      const group = button.parentElement;
      const first = !button.disabled && !groups.has(group);
      button.tabIndex = -1;
      if (!first) return;
      button.tabIndex = 0;
      groups.add(group);
    });
  }

  #handleToggleGroup(event) {
    const { htmx } = this.#environment;
    const button = htmx.closest(event.target, '[data-toggle-value]');
    const unavailable = !button || button.disabled;
    if (unavailable) return;
    const group = htmx.closest(button, '[data-toggle-group]');
    const buttons = Array.from(htmx.findAll(group, '[data-toggle-value]:not(:disabled)'));
    if (event.type === 'keydown') {
      this.#navigateToggleGroup(event, buttons);
      return;
    }
    if (event.type !== 'click') return;
    const selected = button.getAttribute('aria-pressed') !== 'true';
    button.setAttribute('aria-pressed', String(selected));
    buttons.forEach(item => { item.tabIndex = -1; });
    button.tabIndex = 0;
    this.#applyIntegrationFilter();
  }

  #navigateToggleGroup(event, buttons) {
    const keys = ['ArrowLeft', 'ArrowRight', 'Home', 'End'];
    if (!keys.includes(event.key)) return;
    event.preventDefault();
    const current = buttons.indexOf(event.target);
    let next = current + 1;
    if (event.key === 'ArrowLeft') next = current - 1;
    if (event.key === 'Home') next = 0;
    if (event.key === 'End') next = buttons.length - 1;
    const index = (next + buttons.length) % buttons.length;
    buttons.forEach(button => { button.tabIndex = -1; });
    buttons[index].tabIndex = 0;
    buttons[index].focus();
  }

  #initIntegrationFilter() {
    const { document, htmx } = this.#environment;
    const root = htmx.find(document, '#integration-results');
    if (!root) return;
    const find = htmx.find.bind(htmx, root);
    const [input, empty, controls] = ['#integration-search', '[data-integration-empty]', '#integration-filters fieldset'].map(find);
    const buttons = Array.from(htmx.findAll(root, '[data-toggle-value]'));
    const elements = Array.from(htmx.findAll(root, '.integration-card'));
    const cards = elements.map(element => {
      const text = element.textContent.toLowerCase();
      const state = element.getAttribute('data-enabled');
      const card = { element, text, state };
      return card;
    });
    this.#integrationFilter = { input, buttons, empty, cards };
    this.#syncIntegrationStates();
    this.#applyIntegrationFilter();
    controls.disabled = false;
  }

  #syncIntegrationStates() {
    const { buttons, cards } = this.#integrationFilter;
    const states = new Set(cards.map(card => card.state));
    buttons.forEach(button => {
      const available = states.has(button.dataset.toggleValue);
      const restored = available && button.disabled;
      if (restored) button.setAttribute('aria-pressed', 'true');
      button.disabled = !available;
      if (available) return;
      button.setAttribute('aria-pressed', 'false');
      button.tabIndex = -1;
    });
  }

  #scheduleIntegrationFilter(event) {
    if (event.target !== this.#integrationFilter?.input) return;
    clearTimeout(this.#filterTimer);
    this.#filterTimer = setTimeout(() => this.#applyIntegrationFilter(), 100);
  }

  #applyIntegrationFilter() {
    if (!this.#integrationFilter) return;
    clearTimeout(this.#filterTimer);
    const { input, buttons, empty, cards } = this.#integrationFilter;
    const query = input.value.trim().toLowerCase();
    const pressed = buttons.filter(button => button.getAttribute('aria-pressed') === 'true');
    const states = new Set(pressed.map(button => button.dataset.toggleValue));
    cards.forEach(({ element, text, state }) => {
      const matches = states.has(state) && text.includes(query);
      element.hidden = !matches;
    });
    empty.hidden = cards.some(card => !card.element.hidden);
  }

  #initNavigation() {
    const { htmx } = this.#environment;
    if (!this.#sidebar?.matches?.('#sidebar')) return;
    const links = htmx.findAll(this.#sidebar, 'a[href]');
    this.#pageRoutes = Array.from(links, link => {
      const label = link.textContent.trim();
      const path = link.getAttribute('href');
      const route = { label, path };
      return route;
    });
    this.#initSidebar();
    this.#initSearch();
    this.#searchButton.disabled = false;
  }

  #openSearch(event) {
    if (!this.#searchDialog) return;
    const { htmx } = this.#environment;
    const modifier = event.metaKey || event.ctrlKey;
    const plainModifier = modifier && !event.altKey && !event.shiftKey;
    const shortcut = event.type === 'keydown' && plainModifier && event.key.toLowerCase() === 'k';
    const clicked = event.type === 'click' && htmx.closest(event.target, '[data-search-open]');
    const requested = shortcut || clicked;
    if (!requested) return;
    event.preventDefault();
    if (!this.#searchDialog.open) this.#searchDialog.showModal();
    this.#search?.input.focus();
  }

  #initSidebar() {
    const { document, matchMedia } = this.#environment;
    const root = document.documentElement;
    const mobile = matchMedia('(max-width: 1024px)').matches;
    root.dataset.sidebar = mobile ? 'closed' : 'open';
    try {
      const sidebar = this.#environment.localStorage.getItem('switchboard.sidebar');
      if (['open', 'closed'].includes(sidebar)) root.dataset.sidebar = sidebar;
    } catch {}
    this.#syncSidebar();
  }

  #syncSidebar() {
    const { document } = this.#environment;
    const expanded = document.documentElement.dataset.sidebar !== 'closed';
    this.#sidebar.hidden = !expanded;
    this.#sidebarToggle.setAttribute('aria-expanded', String(expanded));
    this.#sidebarToggle.disabled = false;
    this.#sidebarClose.disabled = false;
  }

  #handleSidebar(event) {
    if (!this.#sidebarToggle) return;
    const { document, htmx } = this.#environment;
    const clicked = event.type === 'click' && htmx.closest(event.target, '[data-sidebar-toggle], [data-sidebar-close]');
    const escape = event.type === 'keydown' && event.key === 'Escape' && this.#sidebar.contains(event.target);
    const toggled = clicked || escape;
    if (!toggled) return;
    const closed = escape || !this.#sidebar.hidden;
    const state = closed ? 'closed' : 'open';
    document.documentElement.dataset.sidebar = state;
    this.#syncSidebar();
    if (closed) this.#sidebarToggle.focus();
    try { this.#environment.localStorage.setItem('switchboard.sidebar', state); } catch {}
  }

  #initSearch() {
    const { autoComplete } = this.#environment;
    if (!autoComplete) {
      this.#searchStatus.textContent = 'Search could not load. Use the sidebar links.';
      return;
    }
    const src = this.#searchRoutes.bind(this);
    const keys = ['label'];
    const data = { src, keys };
    const resultsList = { class: 'search-results', maxResults: 10 };
    const resultItem = { class: 'search-result', element: (item, result) => { item.textContent = result.value.label; } };
    const searchEngine = () => ' ';
    const selection = this.#navigateToResult.bind(this);
    const input = { selection };
    const events = { input };
    const config = { selector: '#route-search', wrapper: false, debounce: 200, data, resultsList, resultItem, searchEngine, events };
    this.#search = new autoComplete(config);
    this.#search.input.disabled = false;
  }

  async #searchRoutes(query) {
    this.#searchRequest?.abort();
    this.#searchRequest = new AbortController();
    const { signal } = this.#searchRequest;
    const options = { signal };
    const normalizedQuery = query.toLowerCase();
    const pages = this.#pageRoutes.filter(page => page.label.toLowerCase().includes(normalizedQuery));
    this.#searchStatus.textContent = 'Searching…';
    try {
      const response = await this.#environment.fetch(`/api/navigation?q=${encodeURIComponent(query)}`, options);
      if (!response.ok) throw new Error('Search unavailable');
      const records = await response.json();
      if (query !== this.#search.input.value) throw new Error('Search changed');
      const results = [...pages, ...records];
      this.#searchStatus.textContent = `${results.length} results. Use arrow keys to choose.`;
      return results;
    } catch (error) {
      const obsolete = signal.aborted || query !== this.#search.input.value;
      if (obsolete) throw error;
      this.#searchStatus.textContent = 'Some results are unavailable. Showing page links.';
      return pages;
    }
  }

  #navigateToResult(event) {
    const { location } = this.#environment;
    const destination = new URL(event.detail.selection.value.path, location.href);
    if (destination.origin !== location.origin) return;
    location.assign(destination.href);
  }

  #syncAppearance() {
    const { document, htmx } = this.#environment;
    const root = document.documentElement;
    if (!root) return;
    const controls = htmx.findAll(document, '[data-preference]');
    controls.forEach(control => {
      const value = root.dataset[control.dataset.preference];
      if (control.type === 'radio') control.checked = control.value === value;
      else control.value = value;
      control.disabled = false;
    });
    this.#syncThemePicker();
    if (this.#fontPicker) htmx.find(this.#fontPicker, 'summary').removeAttribute('aria-disabled');
  }

  #changeAppearance(control) {
    if (!control?.matches?.('[data-preference]')) return;
    const key = control.dataset.preference;
    if (!['theme', 'fontSize'].includes(key)) return;
    const unchecked = control.type === 'radio' && !control.checked;
    if (unchecked) return;
    const { document, htmx } = this.#environment;
    const value = control.value;
    const selector = `input[data-preference="${key}"]`;
    const options = control.options ?? htmx.findAll(document, selector);
    const supported = Array.from(options).some(option => option.value === value);
    if (!supported) return;
    document.documentElement.dataset[key] = value;
    if (key === 'theme') this.#syncThemePicker();
    try {
      this.#environment.localStorage.setItem(`switchboard.${key}`, value);
    } catch {}
  }

  #syncThemePicker() {
    const picker = this.#themePicker;
    if (!picker) return;
    const { htmx } = this.#environment;
    const selectors = ['input:checked', 'summary', 'summary [data-theme-name]', 'summary .theme-swatch'];
    const find = htmx.find.bind(htmx, picker);
    const [selected, summary, name, swatch] = selectors.map(find);
    if (!selected) return;
    name.textContent = selected.getAttribute('aria-label');
    swatch.dataset.theme = selected.value;
    summary.removeAttribute('aria-disabled');
  }

  #handleAppearancePickers(event) {
    const pickers = [this.#themePicker, this.#fontPicker];
    pickers.forEach(picker => this.#dismissPicker(picker, event));
  }

  #dismissPicker(picker, event) {
    if (!picker?.open) return;
    const escape = event.type === 'keydown' && event.key === 'Escape';
    const outside = event.type === 'click' && !picker.contains(event.target);
    const selected = event.type === 'click' && event.detail > 0 && event.target.matches('input[data-preference]');
    const dismissed = escape || outside || selected;
    if (!dismissed) return;
    picker.open = false;
    if (outside) return;
    this.#environment.htmx.find(picker, 'summary').focus();
  }

  #dismissClickedFlash(event) {
    const { htmx } = this.#environment;
    const button = htmx.closest(event.target, '[data-dismiss-flash]');
    if (!button) return;
    const banner = htmx.closest(button, '[data-flash]');
    this.dismissFlash(banner);
  }

  #initTimedFlashes() {
    const { document, htmx } = this.#environment;
    const banners = htmx.findAll(document, '[data-auto-dismiss]');
    banners.forEach(banner => this.#scheduleDismiss(banner));
  }

  #scheduleDismiss(banner) {
    this.#cancelDismiss(banner);
    const timer = setTimeout(() => this.dismissFlash(banner, false), 3000);
    this.#dismissTimers.set(banner, timer);
  }

  #cancelDismiss(banner) {
    clearTimeout(this.#dismissTimers.get(banner));
    this.#dismissTimers.delete(banner);
  }

  dismissFlash(banner, restoreFocus = true) {
    if (!banner) return;
    this.#cancelDismiss(banner);
    const { location, history, document, htmx } = this.#environment;
    const key = banner.dataset.flash;
    const url = new URL(location.href);
    const hasFlashParameter = key === 'success' || key === 'error';
    if (hasFlashParameter) {
      url.searchParams.delete(key);
      history.replaceState(history.state, '', url.href);
    }
    const shouldFocus = restoreFocus || banner.contains(document.activeElement);
    const reusable = key === 'refresh';
    if (reusable) banner.hidden = true;
    else banner.remove();
    if (!shouldFocus) return;
    const target = htmx.find(document, '[data-flash]:not([hidden]) [data-dismiss-flash], #page-refresh, #health-refresh button, main');
    target?.focus();
  }

  #updateRefresh(event) {
    const control = event.detail?.requestConfig?.elt ?? event.detail?.elt;
    if (!control?.matches?.('#page-refresh, #health-refresh')) return;
    const completed = event.type === 'htmx:afterRequest' || event.type === 'htmx:sendAbort';
    if (completed) {
      this.#restoreSections(control);
      return;
    }
    if (event.type === 'htmx:beforeRequest') {
      this.#startRefresh(control, event.detail.target);
      return;
    }
    if (event.type === 'htmx:afterSwap') {
      this.#finishRefresh(control, true);
      return;
    }
    const errors = ['htmx:sendError', 'htmx:timeout', 'htmx:responseError', 'htmx:swapError'];
    if (errors.includes(event.type)) this.#finishRefresh(control, false);
  }

  #startRefresh(control, target) {
    const sections = this.#measureSections(target);
    this.#refreshSections.set(control, sections);
    control.setAttribute('aria-busy', 'true');
    this.dismissFlash(this.#refreshStatus, false);
    sections.forEach(({ section, height }) => {
      section.style.setProperty('--refresh-height', `${height}px`);
      section.setAttribute('aria-busy', 'true');
      section.classList.add('section-loading');
    });
  }

  #measureSections(target) {
    const { htmx } = this.#environment;
    const selector = '.stats-grid, .metrics-section, .savings-hero, .card, .table-wrap';
    const candidates = Array.from(htmx.findAll(target, selector));
    const sections = candidates.filter(section => !section.parentElement.closest(selector));
    const measured = sections.map(section => {
      const { height } = section.getBoundingClientRect();
      const busy = section.getAttribute('aria-busy');
      const measurement = { section, height, busy };
      return measurement;
    });
    const visible = measured.filter(({ height }) => height > 0);
    return visible;
  }

  #restoreSections(control) {
    const sections = this.#refreshSections.get(control);
    if (!sections) return;
    sections.forEach(({ section, busy }) => {
      section.classList.remove('section-loading');
      section.style.removeProperty('--refresh-height');
      if (busy === null) section.removeAttribute('aria-busy');
      else section.setAttribute('aria-busy', busy);
    });
    this.#refreshSections.delete(control);
    control.setAttribute('aria-busy', 'false');
  }

  #finishRefresh(control, successful) {
    this.#restoreSections(control);
    let message = 'Could not refresh. Try again.';
    if (successful) message = 'Refreshed.';
    const healthComplete = successful && control.matches('#health-refresh');
    if (healthComplete) message = 'Health checks complete.';
    this.#showRefreshToast(message, successful);
  }

  #showRefreshToast(message, successful) {
    const status = this.#refreshStatus;
    if (!status) return;
    this.#cancelDismiss(status);
    const { htmx } = this.#environment;
    const failed = !successful;
    status.classList.toggle('flash-success', successful);
    status.classList.toggle('flash-error', failed);
    let role = 'status';
    if (failed) role = 'alert';
    status.setAttribute('role', role);
    status.hidden = false;
    htmx.find(status, '[data-flash-message]').textContent = message;
    if (successful) this.#scheduleDismiss(status);
  }
}

UI.getInstance().init();
