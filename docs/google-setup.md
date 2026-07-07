# Google Workspace Setup

Switchboard connects to Google Workspace services (Gmail, Calendar, Drive,
Docs, Sheets, Slides, Forms, Tasks, Chat, Contacts, and Meet) using **your
own** Google Cloud OAuth client. You create one OAuth client, enable the APIs
you need, sign in once, and every selected service is configured from that
single consent.

Because you bring your own client, there is no verification or CASA security
assessment required — the app runs in your own Google Cloud project against
your own account.

The unified setup page lives at **`/integrations/google/setup`** in the web UI.

---

## 1. Create a Google Cloud project

1. Open the [Google Cloud Console](https://console.cloud.google.com/).
2. Create a new project (or reuse an existing one) and select it.

## 2. Enable the APIs you want

Enable an API for each service you plan to connect. In the console go to
**APIs & Services → Library** and enable the ones you need:

| Service   | API to enable                     |
|-----------|-----------------------------------|
| Gmail     | Gmail API                         |
| Calendar  | Google Calendar API               |
| Drive     | Google Drive API                  |
| Docs      | Google Docs API                   |
| Sheets    | Google Sheets API                 |
| Slides    | Google Slides API                 |
| Forms     | Google Forms API                  |
| Tasks     | Google Tasks API                  |
| Chat      | Google Chat API                   |
| Contacts  | People API                        |
| Meet      | Google Meet API                   |

You can enable more later; just re-run the sign-in and Switchboard will request
the additional scopes incrementally (it sends `include_granted_scopes=true`, so
previously granted access is preserved).

## 3. Configure the OAuth consent screen

1. Go to **APIs & Services → OAuth consent screen**.
2. Choose **External** user type.
3. Fill in the required app name and support email.
4. Add yourself as a **Test user** (under the Audience/Test users section).

> **⚠️ 7-day refresh-token expiry in Testing.** While the consent screen is in
> **Testing** status, Google expires refresh tokens after **7 days**. When that
> happens Switchboard's stored tokens stop refreshing and you'll see "Invalid
> token" badges — just sign in again from the setup page.
>
> To avoid the weekly re-auth, **publish your own app** (OAuth consent screen →
> **Publish app**). For a self-hosted, single-user setup you generally do not
> need Google's verification for personal use, and publishing removes the 7-day
> refresh-token cap.

## 4. Create the OAuth client

1. Go to **APIs & Services → Credentials**.
2. Click **Create Credentials → OAuth client ID**.
3. Set the application type to **Web application**.
4. Under **Authorized redirect URIs**, add:

   ```
   http://localhost:3847/api/google/oauth/callback
   ```

   Replace `3847` with your configured port if you run Switchboard on a
   different one (the default is `3847`; the setup page shows the exact URI to
   register).
5. Create the client and copy the **Client ID** and **Client secret**.

## 5. Enter the client in Switchboard

You have two options:

### Option A — Web UI

1. Open `/integrations/google/setup`.
2. Paste the Client ID and Client Secret and save.
3. Check the services you want, click **Sign in with Google**, and complete the
   consent screen. You'll be redirected back and every granted service is
   configured automatically.

### Option B — Environment variables

Set the shared client once and it fans out to all Google services:

```sh
export GOOGLE_OAUTH_CLIENT_ID="123456789-abc.apps.googleusercontent.com"
export GOOGLE_OAUTH_CLIENT_SECRET="GOCSPX-..."
```

Then complete the sign-in from the web UI to obtain tokens.

---

## Scopes requested per service

| Service   | Scopes |
|-----------|--------|
| Gmail     | `https://mail.google.com/` |
| Calendar  | `.../auth/calendar` |
| Drive     | `.../auth/drive` |
| Docs      | `.../auth/documents` |
| Sheets    | `.../auth/spreadsheets` |
| Slides    | `.../auth/presentations` |
| Forms     | `.../auth/forms.body`, `.../auth/forms.responses.readonly` |
| Tasks     | `.../auth/tasks` |
| Chat      | `.../auth/chat.spaces.readonly`, `.../auth/chat.messages` |
| Contacts  | `.../auth/contacts`, `.../auth/contacts.other.readonly`, `.../auth/directory.readonly` |
| Meet      | `.../auth/meetings.space.created`, `.../auth/meetings.space.readonly`, `.../auth/meetings.space.settings` |

(`...` = `https://www.googleapis.com`.) Only the scopes for the services you
select are requested. If Google grants a subset (partial consent), Switchboard
enables only the services whose scopes were actually granted.

---

## How token storage works

Signing in once obtains a single access/refresh token pair covering all granted
scopes. Switchboard writes that pair into the config entry for **each** granted
service, so every Google adapter can refresh independently. On a 401 each
adapter refreshes its own copy of the token and persists the rotated value.

## Troubleshooting

- **"Invalid token" badge after ~7 days** — your consent screen is in Testing
  status. Sign in again, or publish your app (see step 3).
- **`redirect_uri_mismatch`** — the redirect URI registered in the console must
  exactly match `http://localhost:<port>/api/google/oauth/callback`, including
  the port shown on the setup page.
- **A service you enabled isn't connected** — make sure you both enabled its API
  (step 2) and left it checked during sign-in; partial grants only enable
  services whose scopes were approved.
