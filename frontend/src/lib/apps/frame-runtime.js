// The runtime inside a package's sandboxed frame (MA6.1; modular decision 7).
//
// This file is not bundled: package-sandbox.ts imports its text (?raw) and
// writes it as the one inline script of the frame's srcdoc. It runs in an
// opaque origin (sandbox="allow-scripts allow-forms", no allow-same-origin),
// beside the package's own code, so nothing here is trusted: every check
// that matters is the host's (package-sandbox.ts), on the other end of the
// MessagePort. This side only turns the SDK into stubs that post messages.
//
// Protocol (one MessagePort, handed over once by the parent):
//   host -> frame  init {code, ctx}, props {props}, launch {value},
//                  theme {theme, vars, font}, event {sub, value},
//                  reply {id, ok, value | error}, destroy
//   frame -> host  ready, mounted, failed {message}, activate,
//                  call {id, name, args}, sub {sub, name, args}, unsub {sub}
// Keep this file free of the sequence that closes a script element.
(function () {
  'use strict';
  var port = null;
  var nextId = 1;
  var pending = new Map();
  var subs = new Map();
  var handle = null;
  var ctx = null;
  var theme = null;
  var themeListeners = new Set();
  var launch = { value: null, listeners: new Set() };

  function post(msg, transfer) {
    if (port) port.postMessage(msg, transfer || []);
  }

  function errorFrom(e) {
    var err = e && e.name === 'AuthRequiredError' ? new AuthRequiredError(e.message) : new Error((e && e.message) || 'refused by the shell');
    if (e && e.name && e.name !== 'AuthRequiredError') err.name = e.name;
    return err;
  }

  function call(name, args) {
    return new Promise(function (resolve, reject) {
      var id = nextId++;
      pending.set(id, { resolve: resolve, reject: reject });
      try {
        post({ t: 'call', id: id, name: name, args: args });
      } catch (e) {
        pending.delete(id);
        reject(new Error('the call could not cross to the shell: ' + (e && e.message)));
      }
    });
  }

  function AuthRequiredError(message) {
    var err = new Error(message || 'authentication required');
    Object.setPrototypeOf(err, AuthRequiredError.prototype);
    err.name = 'AuthRequiredError';
    return err;
  }
  AuthRequiredError.prototype = Object.create(Error.prototype);
  AuthRequiredError.prototype.constructor = AuthRequiredError;

  function plainHeaders(h) {
    var out = {};
    if (!h) return out;
    if (typeof h.forEach === 'function' && !Array.isArray(h)) {
      h.forEach(function (v, k) { out[String(k)] = String(v); });
    } else if (Array.isArray(h)) {
      h.forEach(function (pair) { out[String(pair[0])] = String(pair[1]); });
    } else {
      Object.keys(h).forEach(function (k) { out[k] = String(h[k]); });
    }
    return out;
  }

  // fetchWithRenewal: the shell makes the request (with the session, which
  // never crosses) only to the routes this package's ceiling names.
  async function fetchWithRenewal(input, init) {
    init = init || {};
    var url = typeof input === 'string' ? input : input && input.url ? input.url : String(input);
    var body = init.body;
    if (body != null && typeof body !== 'string' && !(body instanceof ArrayBuffer)) {
      if (ArrayBuffer.isView(body)) body = body.buffer.slice(body.byteOffset, body.byteOffset + body.byteLength);
      else if (typeof Blob !== 'undefined' && body instanceof Blob) body = await body.arrayBuffer();
      else body = String(body);
    }
    var r = await call('fetchWithRenewal', [url, { method: init.method || 'GET', headers: plainHeaders(init.headers), body: body == null ? null : body }]);
    var nullBody = r.status === 101 || r.status === 204 || r.status === 205 || r.status === 304;
    return new Response(nullBody ? null : r.body, { status: r.status, statusText: r.statusText || '', headers: r.headers || {} });
  }

  function listen(target, type, fn, options) {
    if (typeof fn !== 'function' && !(fn && typeof fn.handleEvent === 'function')) return function () {};
    target.addEventListener(type, fn, options);
    var done = false;
    return function () {
      if (done) return;
      done = true;
      target.removeEventListener(type, fn, options);
    };
  }

  function sdkSatisfies(range, version) {
    var r = /^\^(\d{1,4})\.(\d{1,4})$/.exec(String(range || ''));
    var v = /^(\d+)\.(\d+)\.(\d+)$/.exec(String(version || (ctx && ctx.sdkVersion) || ''));
    if (!r || !v) return false;
    return Number(v[1]) === Number(r[1]) && Number(v[2]) >= Number(r[2]);
  }

  // The launch file: the window's own context, without a URL (a framed app
  // has no network; its file reads go through the bridge when one is granted).
  function launchFileFromAppContext(appContext) {
    var c = appContext || {};
    var path = String(c.filePath || c.sourcePath || '').replace(/^\/+/, '');
    var sourceUrl = String(c.sourceUrl || '');
    if (!path && !sourceUrl) return null;
    return {
      path: path,
      name: String(c.fileName || (path ? path.split('/').pop() : '') || c.windowTitle || ''),
      mediaType: String(c.mediaType || ''),
      url: path ? '' : sourceUrl,
      sourceUrl: path ? '' : sourceUrl,
      openedWith: c.openedWith === 'choice' ? 'choice' : 'default',
    };
  }

  var launchStore = {
    subscribe: function (run) {
      run(launch.value);
      launch.listeners.add(run);
      return function () { launch.listeners.delete(run); };
    },
  };

  function subscribe(name, args, fn) {
    var sub = nextId++;
    subs.set(sub, fn);
    post({ t: 'sub', sub: sub, name: name, args: args });
    var done = false;
    return function () {
      if (done) return;
      done = true;
      subs.delete(sub);
      post({ t: 'unsub', sub: sub });
    };
  }

  function bridged(name) {
    return function () { return call(name, Array.prototype.slice.call(arguments)); };
  }

  function makeSdk() {
    var sdk = {
      SDK_VERSION: ctx.sdkVersion,
      sdkSatisfies: sdkSatisfies,
      buildInfo: function () { return Object.assign({}, ctx.build); },
      AuthRequiredError: AuthRequiredError,
      fetchWithRenewal: fetchWithRenewal,
      onWindowEvent: function (type, fn, options) { return listen(window, type, fn, options); },
      onDocumentEvent: function (type, fn, options) { return listen(document, type, fn, options); },
      repeatEvery: function (fn, ms) {
        if (typeof fn !== 'function' || !Number.isFinite(ms) || ms < 50) throw new Error('repeatEvery needs a function and an interval of at least 50ms');
        var timer = setInterval(fn, ms);
        var done = false;
        return function () {
          if (done) return;
          done = true;
          clearInterval(timer);
        };
      },
      getTheme: function () { return theme; },
      onThemeChange: function (fn) {
        if (typeof fn !== 'function') return function () {};
        themeListeners.add(fn);
        return function () { themeListeners.delete(fn); };
      },
      onSetting: function (key, fn) {
        if (typeof fn !== 'function') return function () {};
        return subscribe('onSetting', [String(key)], fn);
      },
      launchFileFromAppContext: launchFileFromAppContext,
    };
    // The bridged names: each is a stub that posts a message; the shell
    // answers only the names this package's ceiling holds.
    (ctx.bridged || []).forEach(function (name) {
      if (!(name in sdk)) sdk[name] = bridged(name);
    });
    return Object.freeze(sdk);
  }

  function applyTheme(msg) {
    theme = msg.theme || null;
    var root = document.documentElement;
    var vars = msg.vars || {};
    Object.keys(vars).forEach(function (k) {
      if (/^--[a-z0-9-]+$/i.test(k)) root.style.setProperty(k, String(vars[k]));
    });
    if (msg.font) document.body.style.fontFamily = String(msg.font);
    if (msg.scheme) root.style.colorScheme = String(msg.scheme);
    themeListeners.forEach(function (fn) {
      try { fn(theme); } catch (e) { console.error(e); }
    });
  }

  async function mount(code) {
    var url = URL.createObjectURL(new Blob([code], { type: 'text/javascript' }));
    var mod;
    try {
      mod = await import(url);
    } finally {
      URL.revokeObjectURL(url);
    }
    if (typeof mod.mount !== 'function' || mod.abi !== 'choir.app-frontend/module/v1') throw new Error('not a module/v1 module (no mount, or another ABI)');
    var target = document.getElementById('choir-app-root');
    handle = mod.mount(target, {
      sdk: makeSdk(),
      package: ctx.package,
      release: ctx.release,
      windowId: ctx.windowId,
      props: ctx.props || {},
      launchContext: launchStore,
    }) || null;
  }

  function onHostMessage(e) {
    var msg = e.data || {};
    switch (msg.t) {
      case 'init':
        if (ctx) return;
        ctx = msg.ctx || {};
        launch.value = ctx.launch === undefined ? null : ctx.launch;
        if (ctx.theme) applyTheme(ctx.theme);
        mount(String(msg.code || '')).then(function () { post({ t: 'mounted' }); }, function (err) {
          post({ t: 'failed', message: String((err && err.message) || err) });
        });
        return;
      case 'props':
        if (handle && typeof handle.update === 'function') {
          try { handle.update(msg.props || {}); } catch (err) { console.error(err); }
        }
        return;
      case 'launch':
        launch.value = msg.value === undefined ? null : msg.value;
        launch.listeners.forEach(function (run) { run(launch.value); });
        return;
      case 'theme':
        applyTheme(msg);
        return;
      case 'event': {
        var fn = subs.get(msg.sub);
        if (fn) {
          try { fn(msg.value); } catch (err) { console.error(err); }
        }
        return;
      }
      case 'reply': {
        var p = pending.get(msg.id);
        if (!p) return;
        pending.delete(msg.id);
        if (msg.ok) p.resolve(msg.value);
        else p.reject(errorFrom(msg.error));
        return;
      }
      case 'destroy':
        try { if (handle && typeof handle.destroy === 'function') handle.destroy(); } catch (err) { console.error(err); }
        handle = null;
        return;
      default:
    }
  }

  // A press anywhere in the app raises its window (the shell cannot see
  // events inside the frame).
  addEventListener('pointerdown', function () { post({ t: 'activate' }); }, true);

  addEventListener('message', function first(e) {
    if (port || e.source !== parent || !e.ports || !e.ports[0]) return;
    removeEventListener('message', first);
    port = e.ports[0];
    port.onmessage = onHostMessage;
    post({ t: 'ready' });
  });
})();
