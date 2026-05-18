<script>
  import MediaFileApp from './MediaFileApp.svelte';
  export let appContext = {};
</script>

<MediaFileApp kind="video" {appContext} on:authexpired on:openvtext />
