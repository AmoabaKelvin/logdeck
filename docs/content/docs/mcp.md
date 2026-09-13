`logdeck mcp` runs a [Model Context Protocol](https://modelcontextprotocol.io) server over stdio, so an assistant like Claude Desktop, Cursor, or Claude Code can query your LogDeck containers, logs, events, and stats directly. When you opt in, it can also restart a container or run a command.

It is a thin layer over the same HTTP API the web UI and CLI use. It adds no new server and no new way in. Your existing API token decides what it can do.

## Setup

You need the `logdeck` CLI installed (see the [CLI reference](/docs/cli)) and a running LogDeck server. Add one entry to your MCP client's configuration:

```json
{
  "mcpServers": {
    "logdeck": {
      "command": "logdeck",
      "args": ["mcp"],
      "env": {
        "LOGDECK_URL": "https://logdeck.example.com",
        "LOGDECK_TOKEN": "ldk_your_read_token"
      }
    }
  }
}
```

The server reads its connection from `LOGDECK_URL` and `LOGDECK_TOKEN`, or from a saved CLI context. On startup it prints the server it is serving to stderr, and warns if the token is not a scoped `ldk_` API token.

## Capability follows the token

The MCP server never widens what your token can do. The LogDeck server enforces mutations, not the MCP layer, so the safe path is the default:

- A **read-scoped token** (`ldk_`, created in Settings > API Access) can read logs, stats, events, and container details, but every action tool returns a permission error. Hand this one to an assistant by default. See [scoped API tokens](/docs/configuration).
- An **admin token** can do everything an admin can do, including exec, environment variables, and settings. It has the same reach in the MCP server as in the CLI and the web UI. You opt into that by choosing which token to configure.

## Read tools

These are always available and read-only.

- `list_containers`: containers across every host, including removed ones and their health state.
- `get_logs`: recent parsed logs for one container, with tail, level, regex, and time-range filters. Never follows.
- `search_logs`: regex search across many running containers, merged by timestamp.
- `inspect_container`: the full inspect document for one container.
- `list_events`: Docker or Podman events collected over a short bounded window.
- `container_stats` / `host_stats`: live CPU and memory per container, and per-host system stats.
- `list_images` / `list_volumes` / `list_networks`: images, volumes, and networks across hosts.
- `history_search` / `history_status` / `history_containers`: query the persisted log store. Indexed, cursor-paginated, and readable even for containers that no longer exist.

## Action tools

These need an admin token. There are no flags to set. Hand the assistant the token you want it to have, the same way you would for the CLI.

- `start_container` / `stop_container` / `restart_container`: reversible lifecycle actions.
- `remove_container`: remove a container. Irreversible, and marked destructive so clients prompt harder.
- `run_command`: run one non-interactive command in a container and return separate stdout, stderr, and the exit code.
- `get_env` / `set_env`: read and replace a container's environment variables. Values often hold secrets, and a write recreates the container, so it restarts with a new ID.
- `get_settings` / `set_read_only` / `set_log_storage`: read settings, toggle server-wide read-only mode, and change log persistence and its retention caps.
- `set_docker_hosts` / `set_coolify_hosts`: replace the configured hosts. Each takes the complete list instead of merging, so read `get_settings` first.
- `set_auth` / `list_api_tokens` / `create_api_token` / `delete_api_token`: change authentication and manage API tokens. Disabling auth leaves the server open to anyone who can reach it.

Client confirmation prompts are a convenience, not a security boundary. The token scope is the boundary. A read-scoped token cannot use any action tool: the server rejects every mutation and denies the env and settings endpoints outright. Destructive tools carry the protocol's destructive hint, so a well-behaved client prompts harder before running them.

## Notes

- Log tools default to a small tail and cap the number of lines returned, to stay within an assistant's context. `history_search` is cursor-paginated for walking further back.
- `run_command` is non-interactive. It runs one command, returns its output and exit code, and does not attach a terminal. For an interactive shell, use the web terminal.
