<script>
  import MediaFileApp from './MediaFileApp.svelte';
  export let appContext = {};
</script>

<MediaFileApp kind="epub" {appContext} on:authexpired on:openvtext />
