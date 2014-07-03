#!/bin/sh
if [ $# != 1 ] && [ $# != 2 ]; then
  echo "Usage: $0 <kindlr-url> <optional-password>" 1>&2
  exit 1
fi
echo "javascript:{window.location.href=\"$1?u=\"+encodeURIComponent(document.URL)+\"&p=$2\";};void(0);"
