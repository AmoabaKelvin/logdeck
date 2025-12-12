# LogDeck

LogDeck is an open-source, high-performance Docker container monitoring and management tool. Built for speed and ease of use, it provides real-time log streaming, multi-host support, and a beautiful interface for managing your containers.

![LogDeck Landing](./docs/landing.png)
![LogDeck Container View and Logs](./docs/logs.png)

## Features

### Real-Time Log Streaming

- Live log streaming (tail -f style) with play/pause controls
- Historical log viewing with configurable line counts
- Auto-scroll toggle during streaming
- Toggleable timestamps and text wrapping
- Log download in JSON or TXT format

### Advanced Log Filtering & Search

- Full-text search with highlighting
- Search navigation (previous/next match with match counter)
- Filter by log level (TRACE, DEBUG, INFO, WARN, ERROR, FATAL, PANIC)
- Color-coded log level badges for quick visual scanning

### Multi-Host Docker Support

Manage containers across multiple Docker hosts from a single interface:

- Connect to local Unix sockets, remote SSH, or TCP endpoints
- Filter and view containers by specific host or across all hosts
- Real-time container state synchronization
- Secure SSH-based connections with key authentication
- Host-aware operations and log streaming

For detailed configuration, see the [Multi-Host Setup Guide](./multi-host.md).

### Container Lifecycle Management

- Start, stop, restart, and remove containers
- Confirmation dialogs for destructive actions
- Read-only mode for monitoring-only deployments
- Real-time state updates

### Container Discovery & Organization

- Automatic discovery of all running containers
- Group containers by Docker Compose project
- Filter by state (running, exited, paused, restarting, dead)
- Search by container name, ID, or image
- Sort by creation date
- Date range filtering

### Interactive Terminal

- WebSocket-based container terminal access
- Full terminal emulation with XTerm.js
- 10,000 line scrollback history
- Copy-to-clipboard support

### Environment Variables Management

- View and edit container environment variables
- Bulk import from .env files
- Support for quoted values and comments

### Modern UI/UX

- Clean, intuitive dashboard with summary cards
- Dark mode support
- Responsive design (mobile, tablet, desktop)
- URL state persistence for shareable views
- Accessible UI components

### Authentication & Security

- Optional authentication (enable/disable as needed)
- JWT token-based authentication
- Role-based access control
- Read-only mode support
