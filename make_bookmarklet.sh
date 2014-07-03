#!/bin/sh
if [ $# != 1 ]; then
  echo "Usage: $0 <kindlr-url>" 1>&2
  exit 1
fi
echo "javascript:{window.location.href=\"$1?u=\"+encodeURIComponent(document.URL);};void(0);"
