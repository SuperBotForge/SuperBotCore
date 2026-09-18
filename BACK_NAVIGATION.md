# Dialog back navigation

Messenger command definitions opt in with `AllowBack: true`. WASM metadata uses
the optional `allow_back` boolean. Existing plugins keep their current behavior.
The practice plugin 0.38.0 enables it for its messenger commands.

The DSL handler records snapshots before accepted answers, persists them in the
existing Redis dialog JSON, and restores them on Back. No SQL migration is needed.
Text/file prompts, conditional branches and pagination are supported. Back is
handled before the current step's validation. Old navigation buttons are ignored.
At the first step the manager cancels without dispatching the plugin handler and
opens `core.plugins` with the original plugin selected. Immediate and completed
commands have no Back button. Navigation does not undo completed actions.

The button is merged into the last options block, because channel renderers use
that block as the keyboard. Commands must still respect each platform's button
limits, allowing space for one extra button.

After deployment, start a new command to get navigation history. Dialogs started
on older Core versions have no history; Back exits them to the plugin menu.

Validation:

```sh
SUPERBOTGO_PROTOCOL_SCHEMAS=../sdk/protocol/v4/schemas go test ./internal/state ./internal/wasm/adapter ./internal/wasm/protocol ./internal/channel ./internal/channel/discord ./internal/channel/telegram
```

Tests cover branch changes, hidden steps, text validation, upload completion,
pagination, JSON persistence, stale buttons, opt-in behavior, menu cancellation,
and metadata propagation. Live messenger smoke tests are still required after
deployment.
