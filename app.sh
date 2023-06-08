#!/bin/bash

set -e

DBFILE="/data/rarbg_db.sqlite"
if [ ! -f ${DBFILE} ]; then
  >&2 echo "${DBFILE} not found!"
  exit 1
fi

cd /app
ln -s ${DBFILE} db.sqlite
exec ./shadow.bg --port 80 --serve-frontend
