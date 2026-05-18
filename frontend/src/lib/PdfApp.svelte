<script>
  import MediaFileApp from './MediaFileApp.svelte';
  export let appContext = {};
</script>

<MediaFileApp kind="pdf" {appContext} on:authexpired on:openvtext />
