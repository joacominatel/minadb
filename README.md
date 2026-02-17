# minadb

> A blazing-fast, privacy-first terminal database manager for developers and DBAs who refuse to compromise.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev/)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey)]()

---

## 🎯 Vision

**minadb** aims to be the definitive terminal-based database management tool that combines the power of DBeaver with the speed and elegance of modern CLI applications. We believe that working with databases shouldn't require sacrificing performance, privacy, or developer experience.

In a world dominated by bloated GUI applications that consume gigabytes of RAM and track your every move, minadb stands as a lightweight, privacy-respecting alternative that puts you back in control.

---

## 🚀 Mission

**To deliver a complete database management experience in the terminal that is:**

- ⚡ **Faster** than any GUI alternative (startup < 100ms, queries rendered instantly)
- 🔒 **Private by default** (zero telemetry, all data stays local, no internet required)
- 🎨 **Beautiful yet minimal** (intuitive TUI that doesn't get in your way)
- 🔧 **Professional-grade** (feature parity with enterprise tools, without the bloat)
- 🌍 **Universal** (works the same on Linux, macOS, and Windows)

---

## 👥 Who is minadb for?

### Primary Audience

**Developers** who:
- Live in the terminal and prefer keyboard-driven workflows
- Need to quickly inspect databases during development
- Want a lightweight tool that doesn't slow down their machine
- Value privacy and local-first software

**Database Administrators** who:
- Manage multiple database instances daily
- Need fast access to schema information and query execution
- Require a tool that works over SSH without X11 forwarding
- Want reproducible workflows through configuration files

### Use Cases

- **Daily driver**: Replace DBeaver/pgAdmin for 90% of your database work
- **Remote work**: Manage databases over SSH without GUI overhead
- **CI/CD pipelines**: Script database inspections and validations
- **Learning**: Explore database schemas without complexity
- **Quick debugging**: Jump into any database in < 2 seconds

---

## 💎 Core Values

### 1. **Privacy First**
- **Zero telemetry**: We don't collect, transmit, or store any usage data
- **Local-only**: All connections, queries, and history stay on your machine
- **No cloud dependencies**: Works completely offline
- **Transparent**: Open source code you can audit

### 2. **Performance Obsessed**
- **Instant startup**: Launch in under 100ms
- **Memory efficient**: < 50MB RAM footprint vs 500MB+ for GUI tools
- **Lazy loading**: Only fetch data when you need it
- **Optimized queries**: Smart caching and connection pooling
- **Binary size**: Single ~15MB executable vs 200MB+ installers

### 3. **Simplicity in Design**
- **Minimalist TUI**: Clean interface focused on your data, not chrome
- **Intuitive navigation**: Vim-inspired keybindings that feel natural
- **Smart defaults**: Works out of the box with sensible configuration
- **Progressive disclosure**: Advanced features available but not overwhelming

### 4. **Extensibility**
- **Plugin architecture**: Extend with custom database drivers
- **Scriptable**: Integrate into your existing workflows
- **Configuration as code**: Version control your database connections
- **Theme support**: Customize colors to match your terminal

---

## 🎨 Design Philosophy

### Minimalism
Every pixel has a purpose. No unnecessary decorations, no splash screens, no wizards. You open minadb and you're immediately working with your database.

### Keyboard-First
Mouse support is there if you need it, but every action should be achievable faster with the keyboard. Vim users should feel at home.

### Composability
minadb plays well with other Unix tools. Pipe query results to `jq`, `grep`, or whatever you need. Export to standard formats without friction.

### Reliability
Databases are critical infrastructure. minadb handles connection failures gracefully, validates queries before execution, and never loses your work.

---

## 🏗️ Technical Architecture

### Tech Stack

**Core Language**: Go 1.21+
- Native compilation for all platforms
- Excellent performance characteristics
- Strong standard library for database work
- Easy distribution (single binary)

**Terminal UI Framework**
- [`charmbracelet/bubbletea`](https://github.com/charmbracelet/bubbletea) - Elm-inspired TUI framework
- [`charmbracelet/lipgloss`](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [`charmbracelet/bubbles`](https://github.com/charmbracelet/bubbles) - Reusable components

**Database Drivers**
- [`jackc/pgx`](https://github.com/jackc/pgx) - PostgreSQL (native, high-performance)
- [`go-sql-driver/mysql`](https://github.com/go-sql-driver/mysql) - MySQL/MariaDB
- [`mattn/go-sqlite3`](https://github.com/mattn/go-sqlite3) - SQLite
- [`mongodb/mongo-go-driver`](https://go.mongodb.org/mongo-driver) - MongoDB

**Additional Libraries**
- [`alecthomas/chroma`](https://github.com/alecthomas/chroma) - SQL syntax highlighting
- [`spf13/viper`](https://github.com/spf13/viper) - Configuration management
- [`spf13/cobra`](https://github.com/spf13/cobra) - CLI command structure
- [`zalando/go-keyring`](https://github.com/zalando/go-keyring) - Secure credential storage

### Architecture Patterns

```
┌─────────────────────────────────────────┐
│           TUI Layer (bubbletea)         │
│  ┌─────────┬──────────┬──────────────┐ │
│  │Explorer │  Query   │   Results    │ │
│  │  View   │  Editor  │   Viewer     │ │
│  └─────────┴──────────┴──────────────┘ │
└─────────────────────────────────────────┘
                    │
┌─────────────────────────────────────────┐
│         Application Core                │
│  ┌──────────────────────────────────┐  │
│  │   Connection Manager              │  │
│  │   Query Executor                  │  │
│  │   Schema Cache                    │  │
│  │   History Manager                 │  │
│  └──────────────────────────────────┘  │
└─────────────────────────────────────────┘
                    │
┌─────────────────────────────────────────┐
│      Database Abstraction Layer         │
│  ┌──────────────────────────────────┐  │
│  │   Driver Interface                │  │
│  └──────────────────────────────────┘  │
│     │      │       │          │         │
│  ┌──────┬──────┬────────┬──────────┐  │
│  │Postgres│MySQL│SQLite  │ MongoDB  │  │
│  └──────┴──────┴────────┴──────────┘  │
└─────────────────────────────────────────┘
```

**Design Patterns Used**:
- **Repository Pattern**: Database operations abstracted behind interfaces
- **Strategy Pattern**: Pluggable database drivers
- **Observer Pattern**: Reactive UI updates
- **Command Pattern**: Undoable operations
- **Factory Pattern**: Driver instantiation

---

### Phase 1: Foundation (MVP)

- [x] PostgreSQL connection and authentication
- [x] Schema browser (databases → schemas → tables → columns)
- [x] SQL query editor with basic syntax highlighting
- [x] Result viewer with tabular display
- [x] Connection profiles (save/load)
- [x] Query history (session-based)

---

## 🌍 Platform Strategy

### Tier 1 Support (Primary Platforms)
- **Linux**: x86_64, ARM64 (Debian, Ubuntu, Arch, Fedora)
- **macOS**: x86_64 (Intel), ARM64 (Apple Silicon)
- **Windows**: x86_64 (Windows 10+, Windows Server 2019+)

### Tier 2 Support (Community Maintained)
- **FreeBSD**: x86_64
- **OpenBSD**: x86_64

### Distribution Channels

**Package Managers**
```bash
# Homebrew (macOS/Linux)
brew install minadb

# APT (Debian/Ubuntu)
apt install minadb

# Pacman (Arch)
pacman -S minadb

# Scoop (Windows)
scoop install minadb

# Go Install
go install github.com/yourusername/minadb@latest
```

**Binary Releases**
- GitHub Releases with automated builds (GoReleaser)
- Checksums and GPG signatures for verification
- Auto-update mechanism (optional, disabled by default)

---

## 🔐 Security & Privacy

### Data Handling
- **Credentials**: Stored in OS keyring (Keychain/Secret Service/Credential Manager)
- **Query history**: Encrypted at rest with user-controlled keys
- **Connections**: Support for SSL/TLS, SSH tunnels, client certificates
- **Secrets**: Never logged or transmitted

### Compliance
- **GDPR**: No personal data collection
- **No telemetry**: Zero analytics, crash reports only if user opts in
- **Audit trail**: Optional local logging for enterprise users
- **Open source**: Full transparency, security reviews welcome

---

## 🤝 Community & Contribution

### Open Source Commitment
- **License**: MIT (permissive, business-friendly)
- **Governance**: Community-driven with transparent roadmap
- **Documentation**: Comprehensive guides for users and contributors
- **Testing**: High test coverage (target: > 80%)

### Ways to Contribute
- 🐛 Bug reports and feature requests
- 💻 Code contributions (new drivers, features, fixes)
- 📝 Documentation improvements
- 🎨 Theme and UI enhancements
- 🌍 Translations (i18n support planned)
- 💬 Community support (Discord/Discussions)

---

## 🎓 Philosophy

### Why Terminal?
The terminal is the most powerful interface ever created. It's:
- **Universal**: Available on every system, even over SSH
- **Scriptable**: Integrate into any workflow
- **Fast**: No rendering overhead, no GPU needed
- **Accessible**: Works with screen readers and alternative input devices
- **Timeless**: Your skills transfer across decades

### Why Go?
- **Single binary**: No runtime dependencies, easy distribution
- **Cross-compilation**: Build for all platforms from one machine
- **Performance**: Native speed with minimal overhead
- **Standard library**: Excellent database and concurrency support
- **Tooling**: Best-in-class formatter, linter, and test tools

### Why Open Source?
Database tools handle sensitive data. You should never trust a closed-source tool with your production databases. Open source means:
- **Auditability**: See exactly what the code does
- **Security**: Community review catches vulnerabilities
- **Longevity**: Project survives beyond any single maintainer
- **Trust**: No hidden telemetry or data collection

---

## 📜 License

MIT License - see [LICENSE](LICENSE) file for details.

---

**minadb**: Because your database deserves better than bloatware.

*Fast. Private. Terminal-native.*
