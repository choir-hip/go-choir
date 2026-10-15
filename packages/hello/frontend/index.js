// Hello: the S7-0a fixture package (docs/design/s7-0-app-packages-2026-10-14.md).
//
// The frontend ABI is choir.app-frontend/v1: one self-contained ES module
// whose mount(target, ctx) returns { destroy() }. It imports nothing from
// the shell, so a host build of it runs on any computer whatever that
// computer's own divergence.
//
// ctx: { package, version, windowId, data: { get(key), put(key, value) } }

export function mount(target, ctx) {
  const root = document.createElement('section');
  root.setAttribute('data-hello-package', '');
  root.style.cssText = 'display:grid;gap:0.75rem;padding:1.25rem;font:inherit;color:var(--choir-text-primary, inherit);';

  const title = document.createElement('h2');
  title.textContent = 'Hello from a package';
  title.style.margin = '0';

  const about = document.createElement('p');
  about.setAttribute('data-hello-version', '');
  about.textContent = `${ctx.package} ${ctx.version}, loaded by verified digest`;
  about.style.margin = '0';

  const count = document.createElement('output');
  count.setAttribute('data-hello-count', '');
  count.textContent = '0';

  const button = document.createElement('button');
  button.type = 'button';
  button.setAttribute('data-hello-increment', '');
  button.textContent = 'Count';

  let value = 0;
  let destroyed = false;

  async function load() {
    try {
      const stored = await ctx.data.get('count');
      if (!destroyed && typeof stored === 'number') {
        value = stored;
        count.textContent = String(value);
      }
    } catch {
      // A missing datum reads as zero; a failed read leaves the counter at 0.
    }
  }

  async function increment() {
    value += 1;
    count.textContent = String(value);
    await ctx.data.put('count', value);
  }

  button.addEventListener('click', increment);
  root.append(title, about, count, button);
  target.append(root);
  void load();

  return {
    destroy() {
      destroyed = true;
      button.removeEventListener('click', increment);
      root.remove();
    },
  };
}
