#!/bin/sh
set -e

PUID=${PUID:-1000}
PGID=${PGID:-1000}

chmod 666 /dev/dri/render* 2>/dev/null || true

for dir in "${ONYX_CONFIG:-config}" "${ONYX_DATA:-data}" "${ONYX_CACHE:-.cache}" .trash .versions; do
	mkdir -p "$dir"
	chown "$PUID:$PGID" "$dir"
done

exec gosu "$PUID:$PGID" /onyx
