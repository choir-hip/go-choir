<script>
  import MediaFileApp from './MediaFileApp.svelte';
  export let appContext = {};
</script>

<MediaFileApp kind="audio" {appContext} on:authexpired on:openvtext />
