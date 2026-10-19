import { createRequire } from 'node:module';
import { readFileSync, readdirSync } from 'node:fs';

const cliRequire = createRequire(process.argv[1]);
const { default: presetIcons } = cliRequire('@unocss/preset-icons');
const { default: presetWind3 } = cliRequire('@unocss/preset-wind3');
const lucide = cliRequire('@iconify-json/lucide/icons.json');
const simpleIcons = cliRequire('@iconify-json/simple-icons/icons.json');
const themeFile = new URL('./templates/layouts/themes.json', import.meta.url);
const integrationDirectory = new URL('../integrations/', import.meta.url);
const svglBase = 'https://cdn.jsdelivr.net/gh/pheralb/svgl@f9b726cee1d1f3c697a18c880185e89d9a55bb70/static/library/';
const awsLogo = { light: 'aws_light', dark: 'aws_dark' };
const githubLogo = { light: 'github_light', dark: 'github_dark' };
const ollamaLogo = { light: 'ollama_light', dark: 'ollama_dark' };
const vercelLogo = { light: 'vercel', dark: 'vercel_dark' };
const xLogo = { light: 'x', dark: 'x_dark' };
const brandedLogos = {
  aws: awsLogo,
  cloudflare: 'cloudflare', datadog: 'datadog', digitalocean: 'digitalocean',
  figma: 'figma', fly: 'fly', front: 'front',
  gcal: 'google-calendar', gchat: 'google-chat', gcp: 'google-cloud', gdrive: 'drive',
  github: githubLogo,
  gmail: 'gmail', gmeet: 'google-meet', google: 'google',
  gsheets: 'google-sheets', gslides: 'google-slides', kubernetes: 'kubernetes',
  linear: 'linear', microsoft365: 'microsoft-office', notion: 'notion', notionmcp: 'notion',
  ollama: ollamaLogo,
  postgres: 'postgresql', posthog: 'posthog', salesforce: 'salesforce', sentry: 'sentry',
  slack: 'slack', slackmcp: 'slack', stripe: 'stripe', suno: 'suno',
  vercel: vercelLogo,
  x: xLogo,
};
const integrationNames = { notionmcp: 'notion-mcp', webfetch: 'web' };
const simpleLogoAliases = {
  airflow: 'apacheairflow', gdocs: 'googledocs', gforms: 'googleforms',
  gtasks: 'googletasks', paperless: 'paperlessngx', likec4excalidraw: 'excalidraw',
};
const logoSources = {
  launchdarkly: 'https://cdn.jsdelivr.net/gh/gilbarbara/logos@4a5b550414be64d5c007574cd395d910d50e8950/logos/launchdarkly-icon.svg',
  netsuite: 'https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons@519c5834ec021db50db5a0db16a510f5679ae887/svg/netsuite.svg',
  servicenow: 'https://cdn.jsdelivr.net/gh/callback-io/allogo@95790d39b66a2bdb09661ea779fc15361d58f07f/public/logos/servicenow/icon.svg',
  ramp: 'https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons@519c5834ec021db50db5a0db16a510f5679ae887/svg/ramp.svg',
};

const colors = {
  canvas: 'var(--bg)',
  surface: 'var(--surface)',
  hover: 'var(--surface-hover)',
  frame: 'var(--border)',
  subtle: 'var(--border-subtle)',
  content: 'var(--text)',
  secondary: 'var(--text-secondary)',
  muted: 'var(--text-muted)',
  accent: 'var(--accent)',
  'accent-dim': 'var(--accent-dim)',
  'accent-active': 'var(--accent-active)',
  positive: 'var(--green)',
  'positive-dim': 'var(--green-dim)',
  negative: 'var(--red)',
  'negative-dim': 'var(--red-dim)',
  warning: 'var(--yellow)',
  'warning-dim': 'var(--yellow-dim)',
};
const fontFamily = { mono: 'var(--font-mono)' };
const borderRadius = { ui: 'var(--radius)' };
const theme = { colors, fontFamily, borderRadius };
const cardBase = 'bg-surface border border-solid border-subtle rounded-ui p-4';
const card = [cardBase, 'mb-3'];
const integrationLink = [cardBase, 'interactive flex items-center justify-between gap-3 no-underline text-inherit mb-3'];
const integrationCard = [cardBase, 'interactive flex flex-col justify-between gap-3 no-underline text-inherit min-h-[72px] [&[hidden]]:hidden'];
const savingsBucket = [cardBase, 'flex flex-col gap-1'];
const headerBase = 'h-[var(--header-height)] shrink-0 border-b border-b-solid border-b-subtle bg-surface';
const topbar = [headerBase, 'sticky top-0 z-30 flex items-center gap-3 px-4'];
const sidebarBrand = [headerBase, 'flex flex-col justify-center px-4 mb-2'];
const iconButton = 'appearance-none inline-flex shrink-0 items-center justify-center rounded border-none shadow-none bg-transparent text-inherit cursor-pointer transition-colors duration-150 hover:bg-accent-dim active:bg-accent-active disabled:hover:bg-transparent disabled:active:bg-transparent disabled:cursor-not-allowed focus-visible:outline-2 focus-visible:outline-solid focus-visible:outline-current focus-visible:outline-offset-2 motion-reduce:transition-none';
const flashDismiss = [iconButton, 'h-8 w-8'];
const shortcuts = {
  'interactive': 'transition-colors duration-150 ease-in-out motion-reduce:transition-none hover:bg-accent-dim hover:text-accent hover:border-accent focus-visible:bg-accent-dim focus-visible:text-accent focus-visible:outline-2 focus-visible:outline-solid focus-visible:outline-accent focus-visible:outline-offset-2 active:bg-accent-active active:text-accent active:border-accent',
  topbar,
  'icon-button': iconButton,
  'refresh-icon': 'i-lucide:refresh-cw inline-block shrink-0 text-xl motion-safe:[[aria-busy=true]_&]:animate-spin motion-safe:[[aria-busy=true]_&]:animate-delay-150',
  'section-loading': 'after:i-lucide:loader-circle after:text-2xl motion-safe:after:animate-spin',
  'appearance-controls': 'flex items-center justify-end gap-3 shrink-0',
  'search-dialog': 'mx-auto top-[12vh] w-[calc(100%-2rem)] max-w-160 max-h-[80vh] overflow-y-auto rounded-ui border border-solid border-frame bg-surface text-content p-4 shadow-lg',
  'search-results': 'mt-3 max-h-80 overflow-y-auto list-none',
  'search-result': 'interactive block rounded px-3 py-2 text-sm cursor-pointer [&[aria-selected=true]]:bg-accent-dim [&[aria-selected=true]]:text-accent',
  'theme-picker': 'whitespace-nowrap',
  'theme-swatch': 'grid grid-cols-2 gap-0.5 p-1 w-5 h-5 shrink-0 rounded bg-canvas [&>span]:rounded-full',
  'theme-options': 'absolute right-0 top-full z-20 mt-1 p-1 w-72 max-w-[calc(100vw-2rem)] bg-surface text-content border border-solid border-frame rounded-ui shadow-lg',
  'theme-option': 'interactive relative flex items-center gap-2 px-2 py-2 rounded cursor-pointer text-sm',
  'status-success': 'border border-solid border-positive bg-positive-dim text-positive',
  'status-warning': 'border border-solid border-warning bg-warning-dim text-warning',
  'status-error': 'border border-solid border-negative bg-negative-dim text-negative',
  'sidebar': 'sticky top-0 h-screen w-50 min-w-max bg-surface border-r border-r-solid border-r-subtle shrink-0 flex flex-col overflow-y-auto',
  'sidebar-brand': sidebarBrand,
  'sidebar-brand-name': 'text-base font-semibold text-content tracking-[-0.01em]',
  'nav-item': 'interactive flex items-center gap-2 py-2 px-4 text-secondary no-underline text-base border-l-2 border-l-solid border-l-transparent',
  'sidebar-version': 'mt-auto py-3 px-4 border-t border-t-solid border-t-subtle text-muted text-sm font-mono',
  'main': 'flex-1 py-8 px-10 max-w-240 min-w-0',
  'page-title': 'text-xl font-semibold mb-6 tracking-[-0.015em]',
  'section-title': 'text-sm font-semibold uppercase tracking-[0.06em] text-muted mb-2.5',
  'flash': 'py-2 px-3 rounded-ui mb-4 text-base border border-solid',
  'toast-stack': 'fixed bottom-4 right-4 z-50 flex flex-col items-end gap-3 w-max max-w-[min(24rem,calc(100vw-2rem))] pointer-events-none',
  'toast': 'flex items-center justify-between gap-3 max-w-full !mb-0 shadow-lg pointer-events-auto break-words [&>span]:min-w-0',
  'flash-success': 'status-success',
  'flash-error': 'status-error',
  'flash-dismiss': flashDismiss,
  card,
  'card-grid': 'grid gap-3 [&>.card]:mb-0',
  'card-stack': 'flex flex-col gap-3 [&>*]:mb-0',
  'card-error': '[&.stat-card]:status-error [&.metric-card]:status-error [&.efficiency-item]:status-error [&_.stat-label]:text-inherit [&_.metric-label]:text-inherit [&_.efficiency-label]:text-inherit',
  'card-header': 'flex flex-wrap items-center justify-between gap-2 mb-3',
  'card-title': 'text-base font-semibold flex items-center gap-2',
  'badge': 'inline-block text-xs border border-solid py-px px-2 rounded font-medium uppercase tracking-[0.03em]',
  'badge-green': 'status-success',
  'badge-red': 'status-error',
  'badge-yellow': 'status-warning',
  'badge-muted': 'border-frame bg-hover text-muted',
  'form-group': 'mb-2.5',
  'form-label': 'block text-sm text-secondary mb-1 capitalize',
  'form-input': 'w-full bg-canvas border border-solid border-frame rounded text-content py-2 px-3 text-base font-mono transition-colors duration-150 ease-in-out file:btn file:mr-3',
  'input-secret': 'relative flex',
  'secret-toggle': 'interactive absolute right-px top-px bottom-px w-8 flex items-center justify-center bg-transparent border-none text-muted cursor-pointer rounded-l-none rounded-r-3px',
  'toggle': 'relative w-[36px] h-[20px] cursor-pointer inline-block',
  'toggle-group': 'inline-flex shrink-0 rounded-ui border border-solid border-frame overflow-hidden',
  'toggle-group-item': 'interactive inline-flex items-center justify-center px-4 py-2 text-base font-medium font-sans leading-normal bg-surface text-content border-0 rounded-none cursor-pointer [&+&]:border-l [&+&]:border-l-solid [&+&]:border-l-frame aria-pressed:bg-accent-dim aria-pressed:text-accent focus-visible:outline-offset-[-2px] disabled:opacity-50 disabled:cursor-not-allowed disabled:pointer-events-none',
  'btn': 'interactive bg-surface text-content border border-solid border-frame rounded-ui py-2 px-4 text-base font-sans leading-normal cursor-pointer font-medium no-underline inline-flex items-center gap-1.5 disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:bg-surface disabled:hover:text-content disabled:hover:border-frame',
  'btn-sm': 'py-1 px-3 text-sm',
  'metrics-grid': 'card-grid [grid-template-columns:repeat(auto-fit,_minmax(min(100%,_9rem),_1fr))]',
  'stats-grid': 'metrics-grid mb-6',
  'stat-card': cardBase,
  'stat-card-negative': 'card-error',
  'stat-value': 'text-2xl font-bold tabular-nums tracking-[-0.02em]',
  'stat-label': 'text-sm text-muted mt-1',
  'integration-link': integrationLink,
  'integration-grid': 'card-grid [grid-template-columns:repeat(auto-fill,_minmax(min(100%,_14rem),_1fr))] mb-3',
  'integration-card': integrationCard,
  'integration-card-header': 'flex flex-wrap items-center gap-2',
  'integration-card-footer': 'flex flex-wrap items-center gap-2 text-sm text-muted',
  'integration-card-tools': 'text-sm text-muted',
  'tools-rows': 'flex flex-col gap-1 mt-2',
  'tool-row': 'grid [grid-template-columns:minmax(180px,_240px)_1fr] gap-3.5 items-baseline py-1.75 px-0 border-b border-b-solid border-b-subtle',
  'tool-row-name': 'font-mono text-sm text-secondary break-all',
  'tool-row-desc': 'text-sm leading-[1.45] text-muted',
  'tool-row-desc-empty': 'italic',
  'tools-hint': 'text-sm text-muted mt-1',
  'identity-editor': 'pb-0',
  'identity-editor-header': 'items-start',
  'identity-hint': 'mx-0 mt-1 mb-0 max-w-[680px] text-sm leading-normal text-secondary',
  'identity-empty': 'py-3 px-0 border-t border-t-solid border-t-subtle text-sm text-secondary',
  'identity-list': 'border-t border-t-solid border-t-subtle',
  'identity-item': 'py-4 px-0 border-b border-b-solid border-b-subtle',
  'identity-item-new': 'mt-1 border-b-none border-b-current',
  'identity-item-heading': 'flex flex-wrap items-start justify-between gap-4 mb-3.5',
  'identity-name': 'text-base font-semibold',
  'identity-id': 'inline-block mt-0.75 font-mono text-sm text-muted',
  'identity-fields': 'grid grid-cols-2 gap-x-4 gap-y-0',
  'identity-fieldset-heading': 'col-span-full mx-0 mt-1 mb-2 pt-2 border-t border-t-solid border-t-subtle text-sm font-semibold uppercase tracking-[0.06em] text-muted',
  'identity-secret-status': 'block mt-1 font-mono text-sm text-positive',
  'identity-form-actions': 'flex justify-end mt-1',
  'identity-delete': '[&.btn]:text-negative [&.btn]:border-negative [&.btn:is(:hover,:focus-visible,:active)]:status-error [&.btn:focus-visible]:outline-negative',
  'footer': 'mt-8 pt-3 border-t border-t-solid border-t-subtle text-muted text-sm break-words',
  'metrics-section': 'mb-6',
  'metric-card': cardBase,
  'metric-value': 'text-xl font-bold tabular-nums tracking-[-0.02em]',
  'metric-label': 'text-sm text-muted mt-1',
  'metrics-row': 'grid [grid-template-columns:1fr_1fr] gap-3',
  'metrics-half': 'min-w-0',
  'table-wrap': 'overflow-x-auto',
  'metrics-table': 'w-full border-collapse text-base',
  'integration-name-cell': 'capitalize',
  'top-tools-list': 'flex flex-col gap-3',
  'top-tool-item': 'flex items-center gap-2 py-1.5 px-2.5 bg-surface border border-solid border-subtle rounded-[4px] text-base',
  'top-tool-rank': 'w-[18px] h-[18px] bg-accent-dim text-accent rounded-[3px] flex items-center justify-center text-sm font-semibold shrink-0',
  'top-tool-name': 'flex-1 font-mono text-sm overflow-hidden text-ellipsis whitespace-nowrap text-secondary',
  'top-tool-calls': 'tabular-nums font-mono text-muted text-sm',
  'efficiency-grid': 'metrics-grid',
  'efficiency-item': cardBase,
  'efficiency-value': 'text-base font-bold tabular-nums tracking-[-0.01em] block',
  'efficiency-label': 'text-sm text-muted block mt-1',
  'savings-hero': 'bg-accent-dim border border-solid border-accent-dim rounded-ui p-4 mb-6 relative overflow-hidden',
  'savings-hero-head': 'relative mb-3',
  'savings-hero-label': 'text-sm uppercase tracking-[0.08em] text-muted font-medium',
  'savings-hero-value': 'text-4xl font-bold tabular-nums tracking-[-0.03em] text-content leading-[1.1] mt-1',
  'savings-hero-unit': 'text-base font-medium text-muted ml-1.5 tracking-0',
  'savings-hero-sub': 'text-base text-secondary mt-1',
  'savings-hero-dollars': 'text-positive font-medium',
  'savings-buckets': 'relative card-grid [grid-template-columns:repeat(auto-fit,_minmax(min(100%,_10rem),_1fr))]',
  'savings-bucket': savingsBucket,
  'savings-bucket-head': 'flex justify-between items-center gap-2',
  'savings-bucket-label': 'text-sm text-secondary font-medium uppercase tracking-[0.04em]',
  'savings-bucket-pct': 'text-sm tabular-nums text-accent bg-accent-dim py-px px-[6px] rounded-[3px] font-semibold',
  'savings-bucket-value': 'text-xl font-bold tabular-nums tracking-[-0.02em] text-content',
  'savings-bucket-unit': 'text-sm font-medium text-muted ml-1',
  'savings-bucket-meta': 'text-sm text-muted tabular-nums',
  'savings-bucket-blurb': 'text-sm text-muted leading-[1.4] mt-0.5',
  'savings-hero-foot': 'relative text-sm text-muted mt-3 pt-3 border-t border-t-solid border-t-subtle',
  'slack-desc': 'text-secondary text-base leading-[1.6]',
  'slack-steps': 'mt-3 flex flex-col gap-3',
  'slack-step': 'flex gap-2.5 items-start',
  'slack-step-num': 'shrink-0 w-[24px] h-[24px] bg-accent text-white rounded-full flex items-center justify-center text-sm font-semibold',
  'slack-step-detail': 'text-secondary text-base mt-0.5',
  'slack-link': 'interactive text-accent no-underline',
  'slack-code-block': 'bg-canvas border border-solid border-subtle rounded-ui p-2.5 mt-1.5 relative',
  'slack-textarea': 'font-mono text-base resize-y min-h-[56px]',
  'slack-token-meta': 'flex gap-5 text-base text-secondary',
};

const wind3 = presetWind3();
const collections = { lucide: () => lucide };
const icons = presetIcons({ collections });
const presets = [wind3, icons];
const safelist = Object.keys(shortcuts);
const preflights = [{ getCSS: appearanceCSS }, { getCSS: integrationCSS }];

export default { theme, shortcuts, presets, safelist, preflights };

function appearanceCSS() {
  const appearance = JSON.parse(readFileSync(themeFile, 'utf8'));
  const { themes, defaults, fontSizes } = appearance;
  const byID = new Map(themes.map(option => [option.id, option]));
  const light = byID.get(defaults.light);
  const dark = byID.get(defaults.dark);
  const rules = themes.map(option => `[data-theme="${option.id}"]{${themeVariables(option)}}`);
  const sizes = fontSizes.map(size => `[data-font-size="${size}"],.font-size-option[data-size="${size}"]{font-size:${size}%}`);
  const css = [`:root{${themeVariables(light)}}`,
    `@media(prefers-color-scheme:dark){:root:not([data-theme]){${themeVariables(dark)}}}`,
    ...rules, ...sizes].join('\n');
  return css;
}

function themeVariables(option) {
  const colors = Object.entries(option.colors).map(([name, value]) => `--${name}:${value}`);
  const chevron = selectChevron(option.colors['text-muted']);
  const declarations = [`color-scheme:${option.scheme}`, ...colors, `--select-chevron:${chevron}`].join(';');
  return declarations;
}

function selectChevron(color) {
  const body = lucide.icons['chevron-down'].body.replaceAll('currentColor', color);
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${lucide.width} ${lucide.height}">${body}</svg>`;
  const image = `url("data:image/svg+xml,${encodeURIComponent(svg)}")`;
  return image;
}

function integrationCSS() {
  const entries = readdirSync(integrationDirectory, { withFileTypes: true });
  const directories = entries.filter(entry => entry.isDirectory()).map(entry => entry.name);
  const names = [...directories, 'google'].toSorted();
  const rules = names.map(integrationLogo).filter(Boolean);
  const themes = integrationLogoThemes();
  const css = ['/* Integration logos: SVGL, Simple Icons, Iconify Logos, Dashboard Icons, Allogo. Sources in web/style.js. */', ...rules, themes].join('\n');
  process.stdout.write(`Styled logos for ${rules.length}/${names.length} integration entries.\n`);
  return css;
}

function integrationLogo(directory) {
  const name = integrationNames[directory] ?? directory;
  if (!/^[a-z0-9-]+$/.test(name)) throw new Error(`Invalid integration name: ${name}`);
  const icon = brandedLogos[directory];
  if (!icon) {
    const fallback = fallbackIntegrationLogo(directory, name);
    return fallback;
  }
  const variants = logoVariants(icon);
  const light = `url("${svglBase}${variants.light}.svg")`;
  let dark = 'var(--integration-light)';
  if (variants.dark !== variants.light) dark = `url("${svglBase}${variants.dark}.svg")`;
  const rule = `.integration-icon[data-integration="${name}"]{--integration-light:${light};--integration-dark:${dark};mask:none;background:transparent var(--integration-light) center/contain no-repeat}`;
  return rule;
}

function fallbackIntegrationLogo(directory, name) {
  const slug = simpleLogoAliases[directory] ?? directory;
  const icon = simpleIcons.icons[slug];
  if (icon) {
    const image = simpleLogoImage(icon);
    const rule = `.integration-icon[data-integration="${name}"]{--un-icon:${image};color:inherit}`;
    return rule;
  }
  const source = logoSources[directory];
  if (!source) return '';
  const image = `url("${source}")`;
  const rule = `.integration-icon[data-integration="${name}"]{--un-icon:${image};color:inherit}`;
  return rule;
}

function simpleLogoImage(icon) {
  const width = icon.width ?? simpleIcons.width;
  const height = icon.height ?? simpleIcons.height;
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${width} ${height}">${icon.body}</svg>`;
  const encoded = Buffer.from(svg).toString('base64');
  const image = `url("data:image/svg+xml;base64,${encoded}")`;
  return image;
}

function logoVariants(route) {
  if (typeof route === 'string') {
    const variants = { light: route, dark: route };
    return variants;
  }
  return route;
}

function integrationLogoThemes() {
  const config = JSON.parse(readFileSync(themeFile, 'utf8'));
  const darkThemes = config.themes.filter(theme => theme.scheme === 'dark');
  const selectors = darkThemes.map(theme => `[data-theme="${theme.id}"] .integration-icon`);
  const dark = '{background-image:var(--integration-dark)}';
  const css = `${selectors.join(',')}${dark}\n@media(prefers-color-scheme:dark){:root:not([data-theme]) .integration-icon${dark}}`;
  return css;
}
