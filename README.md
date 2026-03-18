# qbee-connect

qbee-connect is a desktop application for managing secure connections to devices managed by the [qbee.io](https://qbee.io) platform. It provides a graphical interface for browsing your device inventory and establishing port-forwarding connections without requiring manual SSH configuration.

## Features

- Browse and search your qbee.io managed device inventory
- Establish secure port-forwarding connections to remote devices
- Manage multiple simultaneous connections
- Cross-platform support for Windows, Linux, and macOS

## How It Works

qbee-connect uses [qbee-cli](https://github.com/qbee-io/qbee-cli) under the hood to communicate with the qbee.io platform. Any functionality available through the qbee.io web UI can also be achieved using qbee-cli from the command line.

## Authentication

Authentication is handled via the **OAuth2 Device Authentication Flow**. When you log in, you will be redirected to the qbee.io platform in your browser to complete authentication. Credentials are stored locally by qbee-cli in `~/.qbee/qbee-cli.json`.

## Installation

Pre-built binaries for **Windows**, **Linux**, and **macOS** are available in the [GitHub Releases](https://github.com/qbee-io/qbee-connect/releases) section.

Download the appropriate package for your operating system and follow the standard installation procedure for your platform.

## License

This project is open-sourced under the [Apache 2.0 License](LICENSE.txt).

## Development

### Prerequisites

Linux:
```
apt install -y libgl1-mesa-dev xorg-dev
```

### Running

```
gow run .
```
