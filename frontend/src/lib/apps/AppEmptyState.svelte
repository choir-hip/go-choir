<script lang="ts">
  export let title = '';
</script>

<section class="app-empty-state" data-app-empty-state>
  {#if title}
    <h3>{title}</h3>
  {/if}
  <slot />
</section>
