#!/usr/bin/env sh
set -eu

curl -i \
  -c /tmp/kotoba-superposition.txt \
  -b /tmp/kotoba-superposition.txt \
  -X POST "https://kotoba-line-ssh-production.up.railway.app/login" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data "username=superposition&password=kotoba"
