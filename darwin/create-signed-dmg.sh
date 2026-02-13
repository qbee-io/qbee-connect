#!/usr/bin/env bash
#
# This script creates a signed and notarized DMG for an app on macOS.
# It performs the following steps:
# 1. Creates a temporary keychain and imports the signing certificate from a base64-encoded .p12 file provided via environment variable.
# 2. Signs the app using the imported certificate.
# 3. Creates a DMG containing the signed app with a custom background and layout.
# 4. Notarizes the DMG using Apple's notarytool and staples the notarization ticket to the DMG.
#
# [1] - https://developer.apple.com/help/account/certificates/create-developer-id-certificates/
# [2] - https://developer.apple.com/documentation/xcode/creating-distribution-signed-code-for-the-mac
# [3] - https://github.com/create-dmg/create-dmg
# [4] - https://developer.apple.com/documentation/security/customizing-the-notarization-workflow#Upload-your-app-to-the-notarization-service

set -eu

# Script Configuration
APP_PATH=qbee-connect.app
DMG_PATH=${1:-Qbee-Connect.dmg}
DMG_BACKGROUND=darwin/dmg-background.png
DMG_VOLUME_NAME="Qbee Connect"
KEYCHAIN_TTL=300 # 5 minutes

# Temporary working directory for keychain and certificate handling
WORKDIR="${TMPDIR:-/tmp}/create-signed-dmg.$$"

# Helper functions
die() { echo "Error: $*" >&2; exit 1; }
info(){ echo "==> $*"; }
cleanup(){ [ -d "$WORKDIR" ] && rm -rf "$WORKDIR"; }
trap cleanup EXIT INT TERM

# Validate required environment variables
if [ -z "${MACOS_SIGN_P12:-}" ]; then
    die "MACOS_SIGN_P12 must be set to the base64-encoded .p12 certificate"
fi

if [ -z "${MACOS_SIGN_PASSWORD:-}" ]; then
    die "MACOS_SIGN_PASSWORD must be set to the password for the .p12 certificate"
fi

if [ -z "${APPLE_CODE_NOTARY_EMAIL:-}" ]; then
    die "APPLE_CODE_NOTARY_EMAIL must be set to the Apple ID for notarization"
fi

if [ -z "${APPLE_CODE_NOTARY_PASSWORD:-}" ]; then
    die "APPLE_CODE_NOTARY_PASSWORD must be set to the password for the Apple ID for notarization"
fi

CERT_P12_PATH=${WORKDIR}/cert.p12
KEYCHAIN_PATH=${WORKDIR}/keychain-db

info "Generating random password for temporary keychain"
KEYCHAIN_PASSWORD=$(openssl rand -base64 32)

info "Creating temporary working directory at $WORKDIR"
mkdir -p "$WORKDIR" || die "Cannot create temp dir"

info "Decoding .p12 certificate and saving to $CERT_P12_PATH"
echo -n "$MACOS_SIGN_P12" | base64 --decode -o $CERT_P12_PATH

info "Extracting signing identity from certificate"
MACOS_SIGN_IDENTITY=$(
    openssl pkcs12 -legacy -in $CERT_P12_PATH -nokeys -passin env:MACOS_SIGN_PASSWORD -clcerts | \
    openssl x509 -noout -subject -nameopt sep_multiline,lname | \
    grep "commonName" | \
    cut -d= -f2)

info $MACOS_SIGN_IDENTITY

info "Extracting Apple Team ID from certificate"
APPLE_TEAM_ID=$(
    openssl pkcs12 -legacy -in $CERT_P12_PATH -nokeys -passin env:MACOS_SIGN_PASSWORD -clcerts | \
    openssl x509 -noout -subject -nameopt sep_multiline,lname | \
    grep "organizationalUnitName" | \
    cut -d= -f2)

info $APPLE_TEAM_ID

info "Creating temporary keychain at $KEYCHAIN_PATH"
security create-keychain -p "$KEYCHAIN_PASSWORD" $KEYCHAIN_PATH

info "Setting keychain settings to prevent locking for $KEYCHAIN_TTL seconds"
security set-keychain-settings -lut $KEYCHAIN_TTL $KEYCHAIN_PATH

info "Unlocking keychain"
security unlock-keychain -p "$KEYCHAIN_PASSWORD" $KEYCHAIN_PATH

info "Importing certificate into keychain"
security import $CERT_P12_PATH -k $KEYCHAIN_PATH -P "$MACOS_SIGN_PASSWORD" -T /usr/bin/codesign

info "Allowing codesign to access the keychain item"
security set-key-partition-list -S apple-tool:,apple: -k "$KEYCHAIN_PASSWORD" $KEYCHAIN_PATH

info "Setting keychain search path to include temporary keychain"
security list-keychains -d user -s $KEYCHAIN_PATH login.keychain

info "Signing $APP_PATH with $MACOS_SIGN_IDENTITY certificate"
codesign --keychain $KEYCHAIN_PATH --force --options runtime --entitlements darwin/entitlements.plist --sign "$MACOS_SIGN_IDENTITY" --timestamp $APP_PATH

info "Creating DMG"
create-dmg \
  --volname "$DMG_VOLUME_NAME" \
  --background "$DMG_BACKGROUND" \
  --no-internet-enable \
  --window-size 640 480 \
  --icon-size 100 \
  --icon "$APP_PATH" 160 310 \
  --app-drop-link 480 310 \
  --hide-extension "$APP_PATH" \
  "$DMG_PATH" \
  "$APP_PATH"

info "Storing notarization credentials in keychain for notarytool"
xcrun notarytool store-credentials "notarytool-password" \
                --keychain "$KEYCHAIN_PATH" \
                --apple-id "$APPLE_CODE_NOTARY_EMAIL" \
                --password "$APPLE_CODE_NOTARY_PASSWORD" \
                --team-id "$APPLE_TEAM_ID"

info "Notarizing DMG with Apple Notary Service"
xcrun notarytool submit --keychain-profile "notarytool-password" --wait "$DMG_PATH"

info "Stapling notarization ticket to DMG"
xcrun stapler staple "$DMG_PATH"