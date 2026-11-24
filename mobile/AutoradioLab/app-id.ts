// The app's own id: what the server's passkey finish route is told (one of its fixed list, internal/appids) and what decides whether this build can make passkeys at all (app-signin.ts passkeySupport).
// Each install has its own (app.json: the production app's literal here, each variant's `env.APP_ID`).
export const APP_ID: string = import.meta.env.APP_ID ?? 'news.choir.autoradio';
