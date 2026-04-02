# SubManager (Proxy Assembly Engine)

SubManager is a powerful, flexible proxy configuration engine and subscription manager. It is designed to aggregate multiple proxy service subscriptions, process and filter nodes, bind them with routing rules, and distribute them securely via custom tokens with a beautiful, modern web UI.

It uses a Go backend backed by a performant SQLite database, featuring an embedded React frontend (Vite/TypeScript) providing a single, cohesive binary to run.

## ✨ Key Features

*   **Multi-Source Aggregation**: Import proxy nodes from multiple Clash-format external subscription links.
*   **Intelligent Routing & Rules**: Import external rule providers (DOMAIN-SUFFIX, IP-CIDR, etc.) and bind them to proxy groups.
*   **Build Profiles (Strategy Engine)**: Define complex proxy groups (`select`, `url-test`, `fallback`, `load-balance`), inject custom base YAML templates, and orchestrate final configurations.
*   **Secure Distribution Tokens**: Generate secure (`/subscribe/xxx`) tokens for users or devices. 
    *   *Security first*: Full token is shown once at creation.
    *   *Per-Token Overrides*: Override subsets of nodes via Regex, rename nodes, modify ports, and inject token-specific rules—all without duplicating Build Profiles.
*   **Asynchronous Prebuilds (Caching)**: Supports async compilation of configs. Cache builds to distribute large configs quickly.
*   **System Alerts & Dependency Validation**: Robust soft-deletion model. Safely track cascading dependencies. If an underlying subscription is removed, affected tokens are flagged via proactive System Alerts instead of breaking silently.
*   **Modern Dashboard**: A fully responsive web interface featuring micro-animations, glassmorphism design, and detailed fetch insights (fetch count, access logs, build diff checks).

## 🏗 Architecture & Stack

**Backend**:
*   **Language**: Go 1.22
*   **Database**: SQLite (`mattn/go-sqlite3`) in WAL mode for robustness and concurrency.
*   **Design**: Clean separation of Domain, Store, Service, API, and Build Engine layers.
*   **Proxy Parsing**: Contains custom parsers for Clash Meta (`*ProxyIR`) logic.

**Frontend**:
*   **Stack**: React 18, TypeScript, Vite.
*   **Styling**: Pure vanilla CSS (`index.css`) built via CSS variables for theme support and custom UI components (Modals, Drawers, Toggle Switches).
*   **Icons**: `lucide-react`.

## 🚀 Getting Started

### Prerequisites

*   **Go** version `1.22` or newer.
*   **Node.js** version `18` or newer (for building the UI).
*   **NPM** or **Yarn**.

### Building the Project

Since the project compiles into a single monolithic binary containing the precompiled frontend assets, you need to build the frontend first.

1. **Build the Frontend:**
   ```bash
   cd frontend
   npm install
   npm run build
   cd ..
   ```

2. **Build the Backend:**
   ```bash
   go build -o submanager main.go
   ```

### Running the App

Start the application by running the compiled binary. By default, it will spawn a server on port `:8080` (or specified port).
```bash
./submanager
```

*   **Dashboard UI**: Visit `http://localhost:8080/` in your browser.
*   **API Core**: Endpoint `/api/*`
*   **Public Subscription Link**: Config downloads go via `http://localhost:8080/subscribe/{token}`

### Storage
The backend creates a persistent SQLite database (`submanager.db`) in a `./data` folder automatically in the directory it's executed from.

## 📖 Core Concepts

1.  **Subscriptions** (`SubscriptionSource`): External links generating proxies (Nodes). Fetched automatically by the background crawler.
2.  **Rules** (`RuleSource`): External links containing rules (e.g. `DOMAIN-SUFFIX,google.com,Proxy`).
3.  **Build Profiles**: Think of this as the "Template" or "Engine". It takes IDs of Subscriptions and Rules, combines them with a YAML template (`TemplateOverride`), and structures how `ProxyGroups` work.
4.  **Download Tokens**: These are the actual instances. A token resolves a Build Profile into an artifact. Tokens can inject node *filters*, *renames*, and *group overrides* on top of their parent Build Profile. Clients fetch these.
