<script lang="ts">
  // The sections packages contribute in their manifest (contributes.settings),
  // rendered from the schema for the base Settings app (slice S6). Values live
  // in the shell's preference store; a field the server refuses shows its
  // reason and keeps the last good value.
  import { onDestroy } from 'svelte';
  import { getSetting, onSetting, setSetting, settingSections, type SettingField } from '../../lib/app-sdk';

  let values: Record<string, unknown> = {};
  let errors: Record<string, string> = {};
  const watching = new Map<string, () => void>();
  const id = (pkg: string, key: string) => `${pkg}.${key}`;

  $: for (const section of $settingSections) {
    for (const field of section.fields) {
      const k = id(section.pkg, field.key);
      if (watching.has(k)) continue;
      values[k] = field.default;
      watching.set(k, onSetting(field.key, (v) => { values = { ...values, [k]: v }; }, section.pkg));
      void getSetting(field.key, section.pkg).then((v) => { values = { ...values, [k]: v }; });
    }
  }
  $: {
    // Fields that left with their package stop being watched.
    const live = new Set($settingSections.flatMap((s) => s.fields.map((f) => id(s.pkg, f.key))));
    for (const [k, off] of watching) if (!live.has(k)) { off(); watching.delete(k); }
  }

  async function save(pkg: string, field: SettingField, value: unknown) {
    const k = id(pkg, field.key);
    const result = await setSetting(field.key, value, pkg);
    errors = { ...errors, [k]: result.ok ? '' : result.error || 'could not save' };
    if (!result.ok) values = { ...values };
  }

  function number(pkg: string, field: SettingField, raw: string) {
    const n = Number(raw);
    if (raw.trim() === '' || !Number.isFinite(n)) {
      errors = { ...errors, [id(pkg, field.key)]: `${field.label} must be a number` };
      return;
    }
    void save(pkg, field, n);
  }

  onDestroy(() => { for (const off of watching.values()) off(); watching.clear(); });
</script>

{#each $settingSections.filter((sec) => sec.pkg !== 'settings') as section (section.pkg + '/' + section.section)}
  <div class="package-section" data-settings-package-section={section.pkg}>
    <h4>{section.section} <span class="package-name">{section.pkg}</span></h4>
    {#each section.fields as field (field.key)}
      {@const k = id(section.pkg, field.key)}
      <label class="package-field" data-settings-field={k}>
        <span>{field.label}</span>
        {#if field.type === 'boolean'}
          <input type="checkbox" checked={values[k] === true} on:change={(e) => save(section.pkg, field, e.currentTarget.checked)} />
        {:else if field.type === 'string' && field.enum?.length}
          <select value={String(values[k] ?? '')} on:change={(e) => save(section.pkg, field, e.currentTarget.value)}>
            {#each field.enum as option}<option value={option}>{option}</option>{/each}
          </select>
        {:else if field.type === 'number'}
          <input type="number" value={Number(values[k] ?? 0)} on:change={(e) => number(section.pkg, field, e.currentTarget.value)} />
        {:else}
          <input type="text" value={String(values[k] ?? '')} maxlength="1024" on:change={(e) => save(section.pkg, field, e.currentTarget.value)} />
        {/if}
      </label>
      {#if errors[k]}<p class="package-error" role="alert">{errors[k]}</p>{/if}
    {/each}
  </div>
{/each}

<style>
  .package-section { display: grid; gap: 0.5rem; margin-top: 0.75rem; }
  h4 { margin: 0; font-size: 0.95rem; }
  .package-name { color: var(--muted, #888); font-weight: 400; font-size: 0.8rem; margin-left: 0.4rem; }
  .package-field { display: flex; justify-content: space-between; align-items: center; gap: 1rem; }
  .package-field input[type='text'], .package-field input[type='number'], .package-field select { min-width: 10rem; }
  .package-error { margin: 0; color: var(--danger, #c0392b); font-size: 0.85rem; }
</style>
