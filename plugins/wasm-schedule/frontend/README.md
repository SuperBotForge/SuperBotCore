# Schedule Plugin Frontend

Static frontend for the `schedule` plugin HTTP trigger.

## Bundle mode

Build one uploadable plugin archive:

```bash
cd ..
make bundle
```

Then upload `schedule-plugin.zip` in the admin UI. The frontend is served by
the core from:

```text
/plugins/schedule/app/
```

In this mode API calls are same-origin and do not need plugin frontend origins.

## Local dev mode

Run it from this directory:

```bash
npm install
npm run dev
```

Register this origin for the `schedule` plugin frontend origins:

```text
http://127.0.0.1:5173
```

The frontend calls:

```text
http://127.0.0.1:4000/api/triggers/http/schedule/api/schedule
```

with `credentials: "include"` and redirects to `/api/auth/tsu/start` when a login is needed.

The local dev server also bridges `/oauth/login` back to the core. This keeps
local TSU.Accounts callbacks working even when the TSU application is still
registered with `http://127.0.0.1:5173/oauth/login`.

Run the Playwright smoke tests while the frontend server is running:

```bash
npm run test:e2e
```
