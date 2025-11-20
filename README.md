# qbee-connect
qbee-connect v2

This project is in PoC state ATM.

## How to test

Authentication is not implemented yet, so you need to do the authentication through qbee-cli

1. Download `qbee-cli` for your OS here: [releases](https://github.com/qbee-io/qbee-cli/releases/latest)
2. Make qbee-cli exectuable
    1. On Windows, rename the binary to `qbee-cli.exe` and put it in your home directory
    2. On MacOS/Linux rename to `qbee-cli`, put in your home directory and run `chmod 755 qbee-cli`
3. Login using a terminal prompt and navigate to your home directory:
    1. Windows: `qbee-cli.exe login -u <your-username>`
    2. MacOS/Linux: `./qbee-cli login -u <your-username>`
4. Download the qbee-connect app for your operating system and start by clicking/double clicking it