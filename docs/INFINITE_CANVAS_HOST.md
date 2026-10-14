# Infinite Canvas host integration

The Sub2API host and the separately built canvas use protocol v2. Deploy the matching pair; pin `VITE_INFINITE_CANVAS_PATH` to an immutable release path when building the Docker image. Preserve earlier static releases for already open v1 pages and rollback.

## Multi-connection release

- Connection settings offer image, text and video sections. Each accepts multiple owned keys, with the first as default. Pasted keys must match an eligible key from the authenticated user's paginated key list. Removing a connection does not delete the key.
- Image connections support OpenAI/Grok/Gemini groups with `allow_image_generation`; video connections currently support Grok with that same existing backend permission. Text uses the existing Responses-compatible platforms: OpenAI, Grok, Kimi, Zhipu, Deepseek, MiniMax and OpenCode Go. No server permission is granted by this UI.
- Store only `{selection: {image: number[], text: number[], video: number[]}, legacyImageKeyId?}` under `sub2api.canvas.connections.<userId>`. Migrate the old image ID from `sub2api.canvas.key.<userId>` without assigning any text/video key. Existing project and media databases are unchanged.
- INIT and UPDATE contain `revision` and `config: {userScope, baseUrl, locale, theme, legacyImageKeyId?, connections}`. Each connection is `{id: '<capability>-<keyId>', apiKeyId, apiKey, name, capability, platform}`. Exact origin, source, session, revision and ownership are checked. Persist accepted choices only after matching ACK. UPDATE preserves the iframe and in-flight work.
- Canvas requests bind to the node's connection/model. No default credential substitution is permitted. Credentials stay in runtime memory; exports/stores contain no key. Removed credentials remain in memory only for already bound tasks until session disposal. The backend continues to enforce revocation, permissions, quota and task ownership on every request.
- Grok video uses JSON `POST /v1/videos/generations`, task-ID polling and authenticated same-origin content downloads. Persist task ID, connection ID and model for refresh recovery. Do not automatically resubmit a failed/ambiguous creation. A missing original key prevents recovery rather than falling back to another key.
- Models come from each connection's `/v1/models`, or `/v1beta/models` for Gemini image connections. Gemini generation/editing uses native non-streaming `/v1beta/models/{model}:generateContent` and `x-goog-api-key`, with the existing Gemini image implementation. Both the host and canvas must be updated together; older canvas releases reject Gemini connections. Name hints are not proof of upstream capability. Explicitly selected aliases may be assigned to a purpose; real generation and billing must be verified on the target upstream. Text image understanding depends on the chosen model.
- The v1 description below documents the previous release only; its reload-on-key-change behavior does not apply to v2.

## Previous v1 implementation

## Administrator switch

Use **Settings → Feature Switches → Infinite Canvas**. The persisted setting is `infinite_canvas_enabled`, default `false`. It uses the existing administrator settings authorization, audit and public-settings/HTML-injection paths. No schema migration is required.

When disabled, the user sidebar hides the entry and direct navigation to `/infinite-canvas` redirects to the appropriate dashboard. The open host page refreshes settings every 30 seconds, on window focus, and before initialization; it removes the iframe when access cannot be confirmed. Background browser timers can be throttled, so this is not a server push guarantee. API keys and ordinary image API calls remain valid under their existing permissions. Closing the workspace cannot cancel or refund an already submitted upstream request.

## Host configuration

Set `VITE_INFINITE_CANVAS_PATH` when building the Sub2API frontend, for example `/canvas/releases/<canvas-release>/`. The default is `/canvas/`. Only the current origin under `/canvas/` is accepted; credentials, query parameters and fragments are rejected. This build setting selects a trusted deployment, not a user-supplied external site.

The host offers the current user's active, unexpired, non-exhausted OpenAI keys whose groups permit image generation. Keys are paginated and checked for ownership. The canvas owns model discovery through the selected key's `/v1/models`, model selection, aliases and generation parameters. The host does not provide a second model selector or assume a default model. Model lists do not certify image capability or upstream availability. The existing gateway remains responsible for group/model permissions, request validation and billing.

No key is created automatically and no image generation is triggered by loading this page. There is no dependency on a deployed canvas for administering the switch or managing keys.

## Full-page interaction

The iframe fills the viewport height beside the existing sidebar. Only this page opts into AppLayout.hideHeader; all other pages retain the site header. On mobile, a floating menu button preserves access to the site sidebar. `AppLayout.fullHeight` opts this page into an unpadded dynamic-viewport flex layout; all other pages keep their default layout.

On first entry the host opens the existing Key selection dialog without choosing the first key automatically. Confirmation validates the key before opening the canvas. The current origin's localStorage remembers only its numeric ID under `sub2api.canvas.key.<userId>`. Re-entry re-fetches this ID through the authenticated API before reconnecting; invalid choices return to selection. Storage write failures do not prevent using an in-memory session.

The floating Connection settings button opens a draft selection over the existing frame. Opening, refreshing the list, cancelling or confirming the same valid key preserves the frame. A different confirmed and validated key sends DISPOSE and replaces the session. Failed draft validation preserves the previous frame. Changing a key reloads the canvas: the dialog reminds users to save and finish generation first. Automatic in-flight coordination is not implemented in protocol v1; the child must persist same-user work safely before any future joint release claims project preservation.

The host rechecks the active key as well as settings every 30 seconds and on window focus. Revoked, expired, foreign or exhausted keys close the session; the gateway enforces every real API request independently. Missing READY/ACK times out after 20 seconds and provides a retry through Connection settings.

## Message protocol v1

Every message includes `protocol: 'sub2api.canvas'`, `version: 1` and `type`. Both applications must check exact origin and window source, rather than using a wildcard target origin. The host only trusts its active iframe and accepts one initialization per frame.

1. Canvas sends `{ protocol, version, type: 'READY' }` to its parent.
2. Host refreshes settings, re-fetches the selected key through the authenticated management API and validates ownership/status. It then sends an `INIT` message with a new opaque `sessionId` and `config`:

   ```ts
   {
     userScope: string, // current user's ID; namespace, not authentication
     apiKeyId: number,
     apiKey: string,    // secret: runtime memory only
     baseUrl: string,   // host origin, not the canvas static-resource prefix
     // No model is supplied: the canvas must discover and select its image model.
     apiFormat: 'openai',
     locale: string,
     theme: 'light' | 'dark'
   }
   ```

3. Canvas sends `{ protocol, version, type: 'ACK', sessionId }` only after safely applying the configuration. ACK must not wait for a model choice or trigger generation; the embedded UI handles that choice after becoming visible. The host reveals the frame only after the matching ACK.
4. Canvas may report `{ protocol, version, type: 'ERROR', sessionId }`. The host closes it and shows a generic error; arbitrary child-provided error content is not rendered or logged.
5. Host sends `{ protocol, version, type: 'DISPOSE', sessionId }` when disconnecting and then destroys the iframe. The canvas must clear its in-memory credentials and cancel pending frontend work. The host does not rely solely on the child handling this message.

No API key appears in the iframe URL. The host never hands the child a JWT, refresh token or upstream administrator credential. Account/Key changes, logout, access refresh failures and unmount invalidate the active frame; late key responses cannot initialize a replacement session. Normal profile refreshes for the same identity do not close the canvas.

## Required work in the separate canvas

- Implement the above handshake before mounting user stores or performing API requests. Directly opening static files without a valid parent session must not restore old credentials.
- Disable the original URL credential import and remove API keys from persistent stores, exports, sync, analytics and logs.
- Namespace project, image, preview, media and generation-log stores by site/user. Treat this as normal-flow account separation, not protection against a shared-device operator or same-origin XSS.
- Adapt the Vite resource base, React Router basename, internal asset paths and refresh fallback to the versioned path.
- Disable remote plugins, custom code, third-party channels/proxies, Agent and unwanted sync/analytics at their execution entry points. Same-origin iframe sandbox attributes do not isolate malicious same-origin scripts from the host; only audited, self-hosted canvas code belongs here.
- Keep the embedded UI full-page, suppress original Base URL/API Key setup, and offer model/parameter selection inside the canvas. Never assume the removed host model field is present.
- Implement image generation/editing and result handling, enforce host-selected API origin, and test actual billing/usage with authorized low-cost requests.

Keep the administrator switch off until the matching canvas is deployed and the joint flow is verified. Nginx should serve versioned static assets and permit same-origin framing only on canvas responses; other application security headers and API routing remain unchanged.

## Verification

The host's unit tests cover settings persistence/defaults/partial updates, public injection, enabled/disabled navigation, user-owned key selection and pagination, origin/source/session validation, delayed responses after logout, same-user profile refresh and disabling an open workspace. Full-page tests also cover first-use selection, user-scoped ID restore, invalid saved choices, draft/cancel behavior, confirmed key changes, key revocation, connection timeout and account changes. Browser fixtures can exercise the handshake without making paid image requests. Actual drawing, project persistence in the canvas, image generation and production deployment require the separate canvas phase.
