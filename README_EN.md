# cdpnetool

English | [简体中文](./README.md)

<p align="center">
  <img src="./docs/assets/images/logo.png" alt="cdpnetool Logo" width="200" />
</p>

> Network Request Interception and Modification Tool Based on Chrome DevTools Protocol

<p align="center">
  <img src="./docs/assets/images/Rules_en.png" alt="cdpnetool Preview" width="800" />
</p>

[![License](https://img.shields.io/badge/license-GPL--3.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/241x/cdpnetool)](https://github.com/241x/cdpnetool/releases)

## Introduction

cdpnetool is a powerful network request interception tool that enables precise control and flexible modification of browser HTTP/HTTPS requests through the Chrome DevTools Protocol. No certificate installation, no code writing required - complete complex network debugging tasks through a visual interface.

**Core Features:**
- ✅ Real-time interception of browser network requests and responses
- ✅ Visual rule configuration with rich matching conditions and modification behaviors
- ✅ Configuration management: multi-configuration saving, import/export, quick switching
- ✅ Event monitoring: DevTools-style detail panel
- ✅ Request preview: full traffic viewing with capture on/off support
- ✅ System settings: language switching (Chinese/English), theme mode, browser configuration
- ✅ No certificate required: direct browser control based on CDP protocol

## Community

[cdpnetool-configs](https://github.com/241x/cdpnetool-configs) - **Find configurations you need** or **Share configurations you created**

## Quick Start

### Installation

**Windows Users:**

Download the latest installer from [Releases](https://github.com/241x/cdpnetool/releases) and run it.

**Other Platforms:**

macOS and Linux platforms require building from source, see [Documentation](./docs/en/02-quick-start.md#building-from-source).

### Basic Usage

1. Launch cdpnetool
2. Click "Launch Browser" button
3. Click "Connect" to connect to the browser
4. Attach the page to intercept in the Targets panel
5. Create rules and enable configuration in the Rules panel
6. View matched requests in the Events panel
7. (Optional) Enable full traffic monitoring in the Network panel

## Documentation

- [Introduction](./docs/en/01-introduction.md) - Learn about cdpnetool's features and use cases
- [Quick Start](./docs/en/02-quick-start.md) - 5-minute getting started tutorial
- [Rule Configuration Reference](./docs/en/03-rule-reference.md) - Complete rule syntax reference
- [FAQ](./docs/en/04-faq.md) - Troubleshooting and solutions

## Use Cases

- 🔧 **Frontend Development**: Mock API data, test error states, debug CORS issues
- 🧪 **Software Testing**: Boundary testing, error simulation, security testing
- 🔍 **API Debugging**: Intercept and analyze encrypted requests, modify parameters to probe behavior
- 🎨 **Personalization**: Ad blocking, network acceleration, feature enhancement

## Tech Stack

- Frontend: React + TypeScript + Vite + TailwindCSS
- Backend: Go + Wails v2
- Protocol: Chrome DevTools Protocol (CDP)
- Database: SQLite
- Internationalization: react-i18next (Chinese/English support)

## Development

```bash
# Clone project
git clone https://github.com/241x/cdpnetool.git
cd cdpnetool

# Install Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Run in development mode
wails dev

# Build production version
wails build
```

For detailed development instructions, see [Quick Start - Building from Source](./docs/en/02-quick-start.md#building-from-source).

## Contributing

Issues and Pull Requests are welcome!

## Discussion Group

Join QQ group [734393923](https://qm.qq.com/q/gHFneBljZ6) to discuss usage and share configurations:

<p align="center">
  <img src="./docs/assets/images/qrcode_group_qq.jpg" alt="QQ Group" width="300" />
</p>

## License

[GPL-3.0 License](LICENSE)

This project is open-sourced under the GNU General Public License v3.0.

## Related Links

- [Chrome DevTools Protocol](https://chromedevtools.github.io/devtools-protocol/)
- [Wails Framework](https://wails.io/)
