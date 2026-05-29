<script lang="ts">
  export let type: 'button' | 'submit' | 'reset' = 'button';
  export let disabled = false;
</script>

<button class="app-button" {type} {disabled} on:click>
  <slot />
</button>
