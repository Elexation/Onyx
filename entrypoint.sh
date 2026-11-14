#!/bin/sh
set -e

PUID=${PUID:-1000}
PGID=${PGID:-1000}

chmod 666 /dev/dri/render* 2>/dev/null || true

for dir in "${ONYX_CONFIG:-config}" "${ONYX_DATA:-data}" "${ONYX_CACHE:-.cache}" .trash .versions; do
	mkdir -p "$dir"
	# Never follow a link as root: one planted by the app user could hand it /etc
	if [ -L "$dir" ]; then
		echo "warning: $dir is a symlink, so the owner of what it points to is left as is" >&2
	fi
	if [ "$(stat -c %u:%g "$dir")" != "$PUID:$PGID" ]; then
		# Some mounts refuse chown (NFS root_squash, read-only); the dir may still be usable
		chown -h "$PUID:$PGID" "$dir" || echo "warning: cannot chown $dir to $PUID:$PGID" >&2
	fi
done

exec gosu "$PUID:$PGID" /onyx
