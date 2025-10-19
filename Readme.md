# LogDeck

Logdeck is an open-source project that aims to be the most intuitive and visually appealing tool for monitoring Docker container logs as well as managing containers.

![LogDeck Landing](./docs/landing.png)
![LogDeck Container View and Logs](./docs/logs.png)

## Getting Started

### Prerequisites

Before running LogDeck locally, ensure you have the following installed:

- **Docker Desktop** - LogDeck needs Docker running to monitor containers
  - macOS: [Download Docker Desktop for Mac](https://www.docker.com/products/docker-desktop)
  - Linux: [Install Docker Engine](https://docs.docker.com/engine/install/)

**For native development (recommended for speed):**
- **Go 1.22+** - [Install Go](https://go.dev/doc/install)
- **Bun** - [Install Bun](https://bun.sh)

**For Docker-based development (recommended for consistency):**
- Only Docker Desktop is required

### Quick Start

#### Option 1: Native Development (Fast & Recommended)

1. **Clone the repository**
   ```bash
   git clone https://github.com/AmoabaKelvin/logdeck.git
   cd logdeck
   ```

2. **Install dependencies**
   ```bash
   make install
   ```

3. **Start the application**
   ```bash
   make dev
   ```

   That's it! 🎉
   - Backend API: http://localhost:8080
   - Frontend: http://localhost:5173

#### Option 2: Docker Compose (Containerized)

1. **Clone the repository**
   ```bash
   git clone https://github.com/AmoabaKelvin/logdeck.git
   cd logdeck
   ```

2. **Start with Docker Compose**
   ```bash
   make dev-docker
   ```

   Or directly:
   ```bash
   docker compose up
   ```

   - Backend API: http://localhost:8080
   - Frontend: http://localhost:5173

### Available Commands

```bash
make help           # Show all available commands
make install        # Install dependencies
make dev            # Run natively (fast hot-reload)
make dev-docker     # Run with Docker Compose
make build          # Build for production
make clean          # Clean build artifacts
make test           # Run tests
make stop-docker    # Stop Docker Compose services
```

### Environment Configuration

You can customize ports and URLs by creating a `.env` file:

```bash
cp .env.example .env
# Edit .env with your preferred settings
```

### 🔥 Hot Reload / Live Reloading

**Both deployment options support live reloading of your changes!**

#### Native Development
- **Frontend (Vite)**: Fast Hot Module Replacement (HMR) - changes appear within milliseconds
- **Backend (Go + Air)**: Automatic rebuild and restart when you save `.go` files
- Air will be automatically installed the first time you run `make dev`

#### Docker Development
- **Frontend (Vite)**: Full HMR support via volume mounts
- **Backend (Go + Air)**: Automatic rebuild and restart via volume mounts
- File changes on your host machine are instantly reflected in containers

**What This Means:**
- Edit frontend code → See changes instantly in browser (no refresh needed for most changes)
- Edit backend code → Server rebuilds and restarts automatically (~2-3 seconds)
- No manual restarts needed during development!

### Troubleshooting

**Air not found after installation?**

Air installs to `$GOPATH/bin` (usually `~/go/bin`). Make sure it's in your PATH:

```bash
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc  # or ~/.zshrc
source ~/.bashrc  # or ~/.zshrc
```

**Docker volume permissions issues on Linux?**

If you encounter permission issues with Docker volumes on Linux, you may need to adjust the ownership:

```bash
sudo chown -R $USER:$USER .
```

## Roadmap for v1.0.0

- [ ] Container discovery

  - [ ] Automatically discover all running containers
  - [ ] Host name, docker version
  - [ ] Host system usage (CPU, memory)
  - [ ] Show container name, image, status, uptime
  - [ ] Real-time updates when containers start/stop
  - [ ] Group containers by project, network, label, etc.
  - [ ] View container details (env vars, volumes, ports, labels, etc.)

- [ ] Log viewing

  - [ ] Real-time log streaming (tail -f style)
  - [ ] Historical logs with option for getting X number of lines
  - [ ] Auto-scroll toggle for streaming logs
  - [ ] Timestamps (toggleable)
  - [ ] Color-coded log levels (ERROR, WARN, INFO, DEBUG)
  - [ ] Download logs as file (downloading both the parsed logs as well as the raw logs)
  - [ ] Pause/resume streaming

- [ ] Basic filtering

  - [ ] Search/filter logs by text
  - [ ] Filter by log level
  - [ ] Date range filtering
  - [ ] Regex support (minimal support for now)

- [ ] Container Life Cycle Management

  - [ ] Start, stop, restart, remove containers (Later on this should be a feature flag that can be enabled/disabled, sometimes we might just want a read only view of the containers and their logs)
  - [ ] View container stats (CPU, memory)
