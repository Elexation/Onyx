#!/bin/sh
set -e

PUID=${PUID:-1000}
PGID=${PGID:-1000}

for dir in "${ONYX_CONFIG:-config}" "${ONYX_DATA:-data}" "${ONYX_CACHE:-.cache}" .trash .versions; do
	mkdir -p "$dir"
	chown "$PUID:$PGID" "$dir"
done

exec su-exec "$PUID:$PGID" /onyx
