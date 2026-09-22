#!/bin/zsh
set -e

SCRIPT_DIR="${0:A:h}"
APP="$SCRIPT_DIR/LW-Manager.app"

if [[ ! -d "$APP" ]]; then
  echo "LW-Manager.app wurde nicht neben dieser Starterdatei gefunden."
  read -k 1 "?Taste drücken zum Schließen …"
  exit 1
fi

xattr -dr com.apple.quarantine "$APP"
chmod +x "$APP/Contents/MacOS/LW-Manager"
open "$APP"
