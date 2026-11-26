#!/bin/sh
set -eu
cat <<'MESSAGE'
Nexora has been installed.
Launch Nexora from your applications menu or run `nexora` as your user.
After Nexora starts, visit http://localhost:18080 in your browser.
The Nexora service runs as your user and is not started by the package manager.
MESSAGE
