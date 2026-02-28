#!/bin/sh
set -e

# Install systemd user service for the installing user.
# nfpms runs post-install as root, so we try to detect the real user.

if command -v systemctl >/dev/null 2>&1; then
    REAL_USER="${SUDO_USER:-$(logname 2>/dev/null || echo root)}"
    REAL_HOME=$(getent passwd "${REAL_USER}" | cut -d: -f6)

    if [ -z "${REAL_HOME}" ]; then
        exit 0
    fi

    UNIT_DIR="${REAL_HOME}/.config/systemd/user"

    mkdir -p "${UNIT_DIR}"
    cp /usr/share/switchboard/switchboard.service "${UNIT_DIR}/switchboard.service"

    if [ "${REAL_USER}" != "root" ]; then
        chown "${REAL_USER}:$(id -gn "${REAL_USER}")" "${UNIT_DIR}/switchboard.service"
        su - "${REAL_USER}" -c "systemctl --user daemon-reload && systemctl --user enable switchboard.service && systemctl --user start switchboard.service" || true
    fi
fi
