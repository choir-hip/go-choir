<script>
  import MediaFileApp from './MediaFileApp.svelte';
  export let appContext = {};
</script>

<MediaFileApp kind="image" {appContext} on:authexpired on:openvtext />
