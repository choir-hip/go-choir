<!--
  FileBrowser — file/directory browser app for the ChoirOS desktop.

  Features:
    - File/directory listing with folder/file icons
    - Breadcrumb navigation with clickable segments
    - Click directory to navigate into it
    - Click text files to open them in Texture
    - Click PDF/EPUB/image/audio/video files to open dedicated media apps
    - Unknown binary files still download
    - New Folder button with inline input (no alert/prompt)
    - Delete with inline confirmation (no confirm())
    - Empty state message
    - Error display for permission/other issues
    - Back/forward navigation history
    - Responsive: works in mobile focus mode with >=44px touch targets

  Data attributes for test targeting:
    data-file-list        — file listing container
    data-file-item        — individual file/directory row
    data-file-icon        — folder/file icon span
    data-file-name        — file/directory name span
    data-file-size        — file size span
    data-breadcrumb       — breadcrumb navigation container
    data-breadcrumb-segment — clickable breadcrumb path segment
    data-new-folder-btn   — new folder button
    data-new-folder-input — inline folder name input
    data-new-folder-confirm — confirm new folder button
    data-delete-btn       — delete button on a file item
    data-delete-confirm   — confirm delete button
    data-delete-cancel    — cancel delete button
    data-empty-state      — empty directory message
    data-error-message    — error message display
    data-nav-back         — back navigation button
    data-nav-forward      — forward navigation button
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { fetchWithRenewal, AuthRequiredError } from './auth.js';
  import { createEventDispatcher } from 'svelte';
  import { mediaRouteForFileName } from './media-utils.js';
  import { addLiveEventListener, currentDeviceId, isOwnLiveEvent, liveEventKind, liveEventPayload } from './live-events.js';
  import { previewFiles, previewFolderEntries } from './public-preview-data';

  const dispatch = createEventDispatcher();
  export let authenticated = false;

  // Auto-focus action for inputs
  function autofocus(node) {
    node.focus();
  }

  // ---- State ----
  let entries = [];
  let currentPath = []; // array of path segments, e.g. ['documents', 'project']
  let loading = false;
  let error = '';

  // Navigation history for back/forward
  let history = [[]]; // start with root path
  let historyIndex = 0;

  // New folder inline input
  let showNewFolderInput = false;
  let newFolderName = '';
  let newFolderError = '';

  // Delete confirmation
  let deleteTarget = null; // { name, type }
  let deleteError = '';

  // Upload state
  let uploadInputEl = null;
  let uploading = false;
  let uploadStatus = '';
  let uploadError = '';

  // ---- API calls ----

  async function fetchDirectory(pathSegments) {
    loading = true;
    error = '';
    entries = [];
    if (!authenticated) {
      const key = pathSegments.join('/');
      entries = key ? (previewFolderEntries[key] || []) : previewFiles;
      loading = false;
      return;
    }
    try {
      const path = pathSegments.length > 0
        ? '/api/files/' + pathSegments.map(encodeURIComponent).join('/')
        : '/api/files';
      const res = await fetchWithRenewal(path);
      if (!res.ok) {
        if (res.status === 401) {
          // Session expired and renewal failed — trigger auth fallback.
          dispatch('authexpired');
          return;
        }
        const body = await res.json().catch(() => ({}));
        if (res.status === 403) {
          error = 'Access denied: you do not have permission to view this directory.';
        } else if (res.status === 404) {
          error = 'Directory not found.';
        } else {
          error = body.error || `Failed to load directory (${res.status})`;
        }
        return;
      }
      const data = await res.json();
      // Sort: directories first, then files, both alphabetically
      entries = (data || []).sort((a, b) => {
        if (a.type === 'directory' && b.type !== 'directory') return -1;
        if (a.type !== 'directory' && b.type === 'directory') return 1;
        return a.name.localeCompare(b.name);
      });
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = 'Failed to load directory. Please try again.';
    } finally {
      loading = false;
    }
  }

  async function createFolder() {
    newFolderError = '';
    if (!authenticated) {
      dispatch('authrequired', { kind: 'file_mutation', appId: 'files', appName: 'Files' });
      return;
    }
    const name = newFolderName.trim();
    if (!name) {
      newFolderError = 'Folder name cannot be empty.';
      return;
    }
    // Check for invalid characters
    if (name.includes('/') || name.includes('\\')) {
      newFolderError = 'Folder name cannot contain / or \\';
      return;
    }

    const path = currentPath.length > 0
      ? '/api/files/' + [...currentPath, name].map(encodeURIComponent).join('/')
      : '/api/files/' + encodeURIComponent(name);

    try {
      const res = await fetchWithRenewal(path, {
        method: 'POST',
        headers: { 'X-Choir-Device': currentDeviceId() },
      });
      if (!res.ok) {
        if (res.status === 401) {
          dispatch('authexpired');
          return;
        }
        const body = await res.json().catch(() => ({}));
        if (res.status === 409) {
          newFolderError = 'A folder with this name already exists.';
        } else {
          newFolderError = body.error || 'Failed to create folder.';
        }
        return;
      }
      // Success — refresh listing
      showNewFolderInput = false;
      newFolderName = '';
      await fetchDirectory(currentPath);
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      newFolderError = 'Failed to create folder.';
    }
  }

  async function deleteItem() {
    if (!deleteTarget) return;
    deleteError = '';
    if (!authenticated) {
      dispatch('authrequired', { kind: 'file_mutation', appId: 'files', appName: 'Files' });
      return;
    }

    const path = currentPath.length > 0
      ? '/api/files/' + [...currentPath, deleteTarget.name].map(encodeURIComponent).join('/')
      : '/api/files/' + encodeURIComponent(deleteTarget.name);

    try {
      const res = await fetchWithRenewal(path, {
        method: 'DELETE',
        headers: { 'X-Choir-Device': currentDeviceId() },
      });
      if (!res.ok) {
        if (res.status === 401) {
          dispatch('authexpired');
          return;
        }
        const body = await res.json().catch(() => ({}));
        deleteError = body.error || 'Failed to delete.';
        return;
      }
      // Success — refresh listing
      deleteTarget = null;
      await fetchDirectory(currentPath);
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      deleteError = 'Failed to delete.';
    }
  }

  async function uploadSelectedFiles(files) {
    uploadError = '';
    uploadStatus = '';
    if (!authenticated) {
      dispatch('authrequired', { kind: 'file_upload', appId: 'files', appName: 'Files' });
      return;
    }
    const selected = Array.from(files || []);
    if (selected.length === 0) return;
    uploading = true;
    try {
      for (const file of selected) {
        if (!file?.name || file.name.includes('/') || file.name.includes('\\')) {
          uploadError = 'File names cannot contain / or \\';
          return;
        }
        const path = currentPath.length > 0
          ? '/api/files/' + [...currentPath, file.name].map(encodeURIComponent).join('/')
          : '/api/files/' + encodeURIComponent(file.name);
        const res = await fetchWithRenewal(path, {
          method: 'PUT',
          headers: {
            ...(file.type ? { 'Content-Type': file.type } : {}),
            'X-Choir-Device': currentDeviceId(),
          },
          body: file,
        });
        if (!res.ok) {
          if (res.status === 401) {
            dispatch('authexpired');
            return;
          }
          const body = await res.json().catch(() => ({}));
          uploadError = body.error || `Failed to upload ${file.name}.`;
          return;
        }
      }
      uploadStatus = selected.length === 1
        ? `Uploaded ${selected[0].name}`
        : `Uploaded ${selected.length} files`;
      await fetchDirectory(currentPath);
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      uploadError = 'Upload failed.';
    } finally {
      uploading = false;
      if (uploadInputEl) uploadInputEl.value = '';
    }
  }

  // ---- Navigation ----

  function navigateTo(pathSegments) {
    if (pathSegments === currentPath) return;
    currentPath = pathSegments;
    showNewFolderInput = false;
    newFolderName = '';
    newFolderError = '';
    deleteTarget = null;
    deleteError = '';
    uploadError = '';
    uploadStatus = '';

    // Update history
    // Trim forward history when navigating
    history = history.slice(0, historyIndex + 1);
    history.push([...pathSegments]);
    historyIndex = history.length - 1;

    fetchDirectory(pathSegments);
  }

  function navigateIntoDirectory(dirName) {
    navigateTo([...currentPath, dirName]);
  }

  function navigateToBreadcrumb(index) {
    navigateTo(currentPath.slice(0, index));
  }

  function goBack() {
    if (historyIndex > 0) {
      historyIndex--;
      currentPath = [...history[historyIndex]];
      showNewFolderInput = false;
      newFolderName = '';
      newFolderError = '';
      deleteTarget = null;
      deleteError = '';
      uploadError = '';
      uploadStatus = '';
      fetchDirectory(currentPath);
    }
  }

  function goForward() {
    if (historyIndex < history.length - 1) {
      historyIndex++;
      currentPath = [...history[historyIndex]];
      showNewFolderInput = false;
      newFolderName = '';
      newFolderError = '';
      deleteTarget = null;
      deleteError = '';
      uploadError = '';
      uploadStatus = '';
      fetchDirectory(currentPath);
    }
  }

  function currentDirectoryPath() {
    return currentPath.join('/');
  }

  function parentPathFor(path) {
    const normalized = String(path || '').replace(/^\/+|\/+$/g, '');
    if (!normalized.includes('/')) return '';
    return normalized.split('/').slice(0, -1).join('/');
  }

  function liveEventTouchesCurrentDirectory(message) {
    if (isOwnLiveEvent(message)) return false;
    const payload = liveEventPayload(message);
    const kind = liveEventKind(message);
    if (kind === 'file.changed') {
      return String(payload.parent_path || '') === currentDirectoryPath();
    }
    if (kind === 'content.item.created') {
      const filePath = payload.file_path || '';
      return filePath && parentPathFor(filePath) === currentDirectoryPath();
    }
    return false;
  }

  function handleFileClick(entry) {
    if (entry.type === 'directory') {
      navigateIntoDirectory(entry.name);
    } else {
      const mediaRoute = mediaRouteForFileName(entry.name);
      if (mediaRoute) {
        const pathSegments = [...currentPath, entry.name];
        dispatch('openmediafile', {
          ...mediaRoute,
          pathSegments,
          filePath: pathSegments.join('/'),
          fileName: entry.name,
        });
        return;
      }

      if (isTextFileName(entry.name)) {
        dispatch('opentextfile', {
          pathSegments: [...currentPath, entry.name],
          fileName: entry.name,
        });
        return;
      }

      // Trigger download for unknown non-text files.
      const path = currentPath.length > 0
        ? '/api/files/' + [...currentPath, entry.name].map(encodeURIComponent).join('/')
        : '/api/files/' + encodeURIComponent(entry.name);

      const a = document.createElement('a');
      a.href = path;
      a.download = entry.name;
      a.style.display = 'none';
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
    }
  }

  function handleImportIntoTexture(entry) {
    dispatch('opentextfile', {
      pathSegments: [...currentPath, entry.name],
      fileName: entry.name,
      importToTexture: true,
    });
  }

  function isTextureShortcutName(name) {
    if (typeof name !== 'string') return false;
    const lower = name.toLowerCase();
    return lower.endsWith('.texture') || lower.endsWith('.texture');
  }

  function isTextFileName(name) {
    const lower = name.toLowerCase();
    if (lower === 'makefile' || lower === 'dockerfile') return true;
    const parts = lower.split('.');
    const ext = parts.length > 1 ? parts.pop() : '';
    if (!ext) return true;
    return [
      'txt', 'md', 'markdown', 'rst', 'org', 'texture', 'texture',
      'json', 'yaml', 'yml', 'toml', 'ini', 'cfg', 'conf',
      'csv', 'tsv', 'log',
      'js', 'jsx', 'ts', 'tsx', 'svelte',
      'go', 'rs', 'py', 'sh', 'bash', 'zsh',
      'css', 'scss', 'html', 'htm', 'xml', 'svg',
      'c', 'h', 'cpp', 'hpp', 'java', 'kt', 'swift', 'rb', 'php', 'pl', 'lua', 'sql',
    ].includes(ext);
  }

  function isTextureImportableDocumentName(name) {
    const lower = String(name || '').toLowerCase();
    return lower.endsWith('.docx') || lower.endsWith('.pdf');
  }

  function fileIconFor(entry) {
    if (entry.type === 'directory') return '📁';
    if (isTextureShortcutName(entry.name)) return '📝';
    const mediaRoute = mediaRouteForFileName(entry.name);
    if (mediaRoute?.appId === 'image') return '🖼️';
    if (mediaRoute?.appId === 'audio') return '🎧';
    if (mediaRoute?.appId === 'video') return '🎬';
    if (mediaRoute?.appId === 'pdf') return '📄';
    if (mediaRoute?.appId === 'epub') return '📚';
    return '📄';
  }

  function handleNewFolderClick() {
    showNewFolderInput = true;
    newFolderName = '';
    newFolderError = '';
  }

  function handleUploadClick() {
    if (!authenticated) {
      dispatch('authrequired', { kind: 'file_upload', appId: 'files', appName: 'Files' });
      return;
    }
    uploadInputEl?.click();
  }

  function cancelNewFolder() {
    showNewFolderInput = false;
    newFolderName = '';
    newFolderError = '';
  }

  function handleNewFolderKeydown(event) {
    if (event.key === 'Enter') {
      createFolder();
    } else if (event.key === 'Escape') {
      cancelNewFolder();
    }
  }

  function startDelete(entry) {
    if (!authenticated) {
      dispatch('authrequired', { kind: 'file_mutation', appId: 'files', appName: 'Files' });
      return;
    }
    deleteTarget = { name: entry.name, type: entry.type };
    deleteError = '';
  }

  function cancelDelete() {
    deleteTarget = null;
    deleteError = '';
  }

  function formatFileSize(bytes) {
    if (bytes === 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    const val = (bytes / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1);
    return `${val} ${units[i]}`;
  }

  // Can go back/forward?
  $: canGoBack = historyIndex > 0;
  $: canGoForward = historyIndex < history.length - 1;

  // ---- Lifecycle ----

  onMount(() => {
    fetchDirectory([]);
    const removeLiveListener = addLiveEventListener((message) => {
      if (liveEventTouchesCurrentDirectory(message)) {
        void fetchDirectory(currentPath);
      }
    });
    return () => {
      removeLiveListener();
    };
  });
</script>

<div class="file-browser" data-file-list>
  <!-- Toolbar: breadcrumb + actions -->
  <div class="toolbar">
    <div class="nav-buttons">
      <button
        class="nav-btn"
        data-nav-back
        on:click={goBack}
        disabled={!canGoBack}
        title="Back"
        aria-label="Go back"
      >
        ←
      </button>
      <button
        class="nav-btn"
        data-nav-forward
        on:click={goForward}
        disabled={!canGoForward}
        title="Forward"
        aria-label="Go forward"
      >
        →
      </button>
    </div>

    <!-- Breadcrumb -->
    <div class="breadcrumb" data-breadcrumb>
      <button
        class="breadcrumb-segment"
        data-breadcrumb-segment
        on:click={() => navigateToBreadcrumb(0)}
        aria-label="Root directory"
      >
        Root
      </button>
      {#each currentPath as segment, i}
        <span class="breadcrumb-sep">/</span>
        <button
          class="breadcrumb-segment"
          data-breadcrumb-segment
          on:click={() => navigateToBreadcrumb(i + 1)}
          aria-label="Navigate to {segment}"
        >
          {segment}
        </button>
      {/each}
    </div>

    <button
      class="action-btn new-folder-btn"
      data-new-folder-btn
      on:click={handleNewFolderClick}
      title="New Folder"
      aria-label="Create new folder"
    >
      + Folder
    </button>
    <button
      class="action-btn upload-btn"
      data-upload-btn
      on:click={handleUploadClick}
      disabled={uploading}
      title="Upload files"
      aria-label="Upload files"
    >
      {uploading ? 'Uploading...' : 'Upload'}
    </button>
    <input
      class="upload-input"
      data-upload-input
      bind:this={uploadInputEl}
      type="file"
      multiple
      on:change={(event) => uploadSelectedFiles(event.currentTarget.files)}
      aria-label="Choose files to upload"
    />
  </div>

  <!-- New folder inline input -->
  {#if showNewFolderInput}
    <div class="inline-input-row">
      <span class="inline-icon">📁</span>
      <input
        type="text"
        class="folder-name-input"
        data-new-folder-input
        use:autofocus
        bind:value={newFolderName}
        on:keydown={handleNewFolderKeydown}
        placeholder="Folder name"
        aria-label="New folder name"
      />
      <button
        class="inline-confirm-btn"
        data-new-folder-confirm
        on:click={createFolder}
        title="Create folder"
        aria-label="Confirm create folder"
      >
        ✓
      </button>
      <button
        class="inline-cancel-btn"
        data-new-folder-cancel
        on:click={cancelNewFolder}
        title="Cancel"
        aria-label="Cancel create folder"
      >
        ✕
      </button>
      {#if newFolderError}
        <span class="inline-error">{newFolderError}</span>
      {/if}
    </div>
  {/if}

  <!-- Error display -->
  {#if error}
    <div class="error-message" data-error-message role="alert">
      <span class="error-icon">⚠️</span>
      {error}
    </div>
  {/if}

  <!-- Delete error display -->
  {#if deleteError}
    <div class="error-message" data-error-message role="alert">
      <span class="error-icon">⚠️</span>
      {deleteError}
    </div>
  {/if}

  {#if uploadStatus}
    <div class="status-message" data-upload-status role="status">
      {uploadStatus}
    </div>
  {/if}

  {#if uploadError}
    <div class="error-message" data-upload-error role="alert">
      <span class="error-icon">⚠️</span>
      {uploadError}
    </div>
  {/if}

  <!-- Loading state -->
  {#if loading}
    <div class="loading-state">
      <span class="loading-spinner"></span>
      Loading...
    </div>
  {:else if !error}
    <!-- Empty state -->
    {#if entries.length === 0}
      <div class="empty-state" data-empty-state>
        <span class="empty-icon">📂</span>
        This folder is empty
      </div>
    {:else}
      <!-- File listing -->
      <div class="file-listing">
        {#each entries as entry (entry.name)}
          {#if deleteTarget && deleteTarget.name === entry.name}
            <!-- Delete confirmation row -->
            <div class="file-item delete-confirm-row" data-file-item data-entry-type={entry.type}>
              <span class="file-icon" data-file-icon>{fileIconFor(entry)}</span>
              <span class="file-name" data-file-name>{entry.name}</span>
              <span class="delete-prompt">Delete?</span>
              <button
                class="delete-confirm-btn"
                data-delete-confirm
                on:click={deleteItem}
                aria-label="Confirm delete {entry.name}"
              >
                Yes
              </button>
              <button
                class="delete-cancel-btn"
                data-delete-cancel
                on:click={cancelDelete}
                aria-label="Cancel delete"
              >
                No
              </button>
            </div>
          {:else}
            <!-- Normal file/directory row -->
            <!-- svelte-ignore a11y-click-events-have-key-events -->
            <!-- svelte-ignore a11y-no-static-element-interactions -->
            <div
              class="file-item"
              data-file-item
              data-entry-type={entry.type}
              on:click={() => handleFileClick(entry)}
            >
              <span class="file-icon" data-file-icon>{fileIconFor(entry)}</span>
              <span class="file-name" data-file-name>{entry.name}</span>
              {#if entry.type === 'file'}
                <span class="file-size" data-file-size>{formatFileSize(entry.size)}</span>
                {#if isTextureImportableDocumentName(entry.name)}
                  <button
                    class="import-texture-btn"
                    data-import-texture-btn
                    on:click|stopPropagation={() => handleImportIntoTexture(entry)}
                    title="Open {entry.name} in Texture"
                    aria-label="Open {entry.name} in Texture"
                  >
                    Texture
                  </button>
                {/if}
              {/if}
              <button
                class="delete-btn"
                data-delete-btn
                on:click|stopPropagation={() => startDelete(entry)}
                title="Delete {entry.name}"
                aria-label="Delete {entry.name}"
              >
                🗑
              </button>
            </div>
          {/if}
        {/each}
      </div>
    {/if}
  {/if}
</div>

<style>
  .file-browser {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
    font-size: 0.85rem;
  }

  /* ---- Toolbar ---- */
  .toolbar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    background: var(--choir-state-selected);
    border-bottom: 1px solid var(--choir-border-strong);
    flex-shrink: 0;
    flex-wrap: wrap;
  }

  .nav-buttons {
    display: flex;
    gap: 2px;
    flex-shrink: 0;
  }

  .nav-btn {
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: 1px solid var(--choir-border);
    border-radius: 4px;
    color: var(--choir-text-accent);
    cursor: pointer;
    font-size: 1rem;
    transition: background 0.15s;
  }

  .nav-btn:hover:not(:disabled) {
    background: color-mix(in srgb, var(--choir-text-primary) 8%, transparent);
  }

  .nav-btn:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }

  /* ---- Breadcrumb ---- */
  .breadcrumb {
    display: flex;
    align-items: center;
    gap: 2px;
    flex: 1;
    min-width: 0;
    overflow-x: auto;
    scrollbar-width: none;
  }

  .breadcrumb::-webkit-scrollbar {
    display: none;
  }

  .breadcrumb-segment {
    background: transparent;
    border: none;
    color: var(--choir-text-accent);
    cursor: pointer;
    font-size: 0.8rem;
    padding: 2px 6px;
    border-radius: 3px;
    white-space: nowrap;
    transition: color 0.15s, background 0.15s;
  }

  .breadcrumb-segment:hover {
    color: var(--choir-text-accent);
    background: color-mix(in srgb, var(--choir-text-primary) 6%, transparent);
  }

  .breadcrumb-sep {
    color: var(--choir-text-subtle);
    font-size: 0.75rem;
    flex-shrink: 0;
  }

  /* ---- Action buttons ---- */
  .action-btn {
    padding: 6px 12px;
    background: var(--choir-state-hover);
    border: 1px solid var(--choir-border-strong);
    border-radius: 4px;
    color: var(--choir-text-accent);
    cursor: pointer;
    font-size: 0.8rem;
    white-space: nowrap;
    transition: background 0.15s;
  }

  .action-btn:hover {
    background: var(--choir-state-selected);
  }

  .action-btn:disabled {
    cursor: not-allowed;
    opacity: 0.55;
  }

  .upload-input {
    position: absolute;
    width: 1px;
    height: 1px;
    opacity: 0;
    pointer-events: none;
  }

  /* ---- Inline input (new folder) ---- */
  .inline-input-row {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    background: var(--choir-state-hover);
    border-bottom: 1px solid var(--choir-border-strong);
    flex-shrink: 0;
  }

  .inline-icon {
    font-size: 1.1rem;
  }

  .folder-name-input {
    flex: 1;
    padding: 6px 10px;
    background: var(--choir-state-selected);
    border: 1px solid var(--choir-border);
    border-radius: 4px;
    color: var(--choir-text-primary);
    font-size: 0.85rem;
    min-width: 0;
  }

  .folder-name-input:focus {
    outline: none;
    border-color: var(--choir-border-strong);
  }

  .inline-confirm-btn,
  .inline-cancel-btn {
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.85rem;
    transition: background 0.15s;
  }

  .inline-confirm-btn {
    background: var(--choir-status-success-soft);
    color: var(--choir-status-success);
  }

  .inline-confirm-btn:hover {
    background: var(--choir-status-success-soft);
  }

  .inline-cancel-btn {
    background: var(--choir-status-danger-soft);
    color: var(--choir-status-danger);
  }

  .inline-cancel-btn:hover {
    background: var(--choir-status-danger-soft);
  }

  .inline-error {
    color: var(--choir-status-danger);
    font-size: 0.8rem;
    white-space: nowrap;
  }

  /* ---- Error message ---- */
  .error-message {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px 16px;
    background: var(--choir-status-danger-soft);
    border-bottom: 1px solid var(--choir-status-danger);
    color: var(--choir-status-danger);
    font-size: 0.85rem;
    flex-shrink: 0;
  }

  .error-icon {
    font-size: 1rem;
  }

  .status-message {
    padding: 10px 16px;
    background: var(--choir-status-success-soft);
    border-bottom: 1px solid var(--choir-status-success);
    color: var(--choir-status-success);
    font-size: 0.85rem;
    flex-shrink: 0;
  }

  /* ---- Loading state ---- */
  .loading-state {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 32px;
    color: var(--choir-text-muted);
    font-size: 0.9rem;
  }

  .loading-spinner {
    display: inline-block;
    width: 16px;
    height: 16px;
    border: 2px solid var(--choir-border);
    border-top-color: var(--choir-border-strong);
    border-radius: 50%;
    animation: spin 0.6s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  /* ---- Empty state ---- */
  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 40px 16px;
    color: var(--choir-text-muted);
    font-size: 0.9rem;
  }

  .empty-icon {
    font-size: 2rem;
    opacity: 0.5;
  }

  /* ---- File listing ---- */
  .file-listing {
    flex: 1;
    overflow-y: auto;
    padding: 4px 0;
    scrollbar-width: thin;
    scrollbar-color: var(--choir-border) transparent;
  }

  .file-listing::-webkit-scrollbar {
    width: 6px;
  }

  .file-listing::-webkit-scrollbar-thumb {
    background: var(--choir-surface-inset);
    border-radius: 3px;
  }

  .file-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 16px;
    min-height: 44px; /* Touch target size for mobile (VAL-FILES-018) */
    cursor: pointer;
    transition: background 0.1s;
    position: relative;
  }

  .file-item:hover {
    background: color-mix(in srgb, var(--choir-text-primary) 4%, transparent);
  }

  .file-icon {
    font-size: 1.2rem;
    flex-shrink: 0;
    width: 24px;
    text-align: center;
  }

  .file-name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--choir-text-accent);
    font-size: 0.85rem;
  }

  .file-size {
    color: var(--choir-text-subtle);
    font-size: 0.75rem;
    flex-shrink: 0;
    margin-right: 4px;
  }

  .import-texture-btn {
    flex-shrink: 0;
    min-height: 28px;
    padding: 0 10px;
    border: 1px solid var(--choir-border);
    border-radius: 4px;
    background: color-mix(in srgb, var(--choir-text-primary) 8%, transparent);
    color: var(--choir-text-accent);
    font-size: 0.72rem;
    font-weight: 700;
    cursor: pointer;
  }

  .import-texture-btn:hover,
  .import-texture-btn:focus-visible {
    background: var(--choir-state-hover);
  }

  /* Delete button - hidden until hover, always visible on mobile */
  .delete-btn {
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.9rem;
    opacity: 0;
    transition: opacity 0.15s, background 0.15s;
    flex-shrink: 0;
  }

  .file-item:hover .delete-btn {
    opacity: 0.6;
  }

  .delete-btn:hover {
    opacity: 1 !important;
    background: var(--choir-status-danger-soft);
  }

  /* ---- Delete confirmation row ---- */
  .delete-confirm-row {
    background: var(--choir-status-danger-soft);
    cursor: default;
  }

  .delete-prompt {
    color: var(--choir-status-danger);
    font-size: 0.8rem;
    white-space: nowrap;
  }

  .delete-confirm-btn,
  .delete-cancel-btn {
    padding: 4px 12px;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.8rem;
    transition: background 0.15s;
  }

  .delete-confirm-btn {
    background: var(--choir-status-danger-soft);
    color: var(--choir-status-danger);
  }

  .delete-confirm-btn:hover {
    background: var(--choir-status-danger-soft);
  }

  .delete-cancel-btn {
    background: color-mix(in srgb, var(--choir-text-primary) 8%, transparent);
    color: var(--choir-text-muted);
  }

  .delete-cancel-btn:hover {
    background: var(--choir-surface-card);
  }

  /* ---- Mobile responsive ---- */
  @media (max-width: 768px) {
    .toolbar {
      padding: 6px 8px;
      gap: 6px;
    }

    .breadcrumb-segment {
      font-size: 0.75rem;
      padding: 2px 4px;
    }

    .action-btn {
      padding: 6px 8px;
      font-size: 0.75rem;
    }

    .file-item {
      padding: 10px 12px;
    }

    /* Always show delete button on mobile (no hover) */
    .delete-btn {
      opacity: 0.5;
    }

    .file-name {
      font-size: 0.9rem;
    }
  }
</style>
