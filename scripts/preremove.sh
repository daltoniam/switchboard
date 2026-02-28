#!/bin/sh
set -e

if command -v systemctl >/dev/null 2>&1; then
    REAL_USER="${SUDO_USER:-$(logname 2>/dev/null || echo root)}"
    REAL_HOME=$(getent passwd "${REAL_USER}" | cut -d: -f6)

    if [ -z "${REAL_HOME}" ]; then
        exit 0
    fi

    UNIT_FILE="${REAL_HOME}/.config/systemd/user/switchboard.service"

    if [ "${REAL_USER}" != "root" ]; then
        su - "${REAL_USER}" -c "systemctl --user stop switchboard.service 2>/dev/null; systemctl --user disable switchboard.service 2>/dev/null" || true
    fi

    rm -f "${UNIT_FILE}"

    if [ "${REAL_USER}" != "root" ]; then
        su - "${REAL_USER}" -c "systemctl --user daemon-reload" || true
    fi
fi
