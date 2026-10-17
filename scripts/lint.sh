#!/bin/sh
# Run from the repository root. Containers receive the host's tracked file list.
set -eu

lint_files=$(mktemp)
trap 'rm -f "$lint_files"' EXIT
if test -n "${CATI_LINT_FILES:-}"
then cp "$CATI_LINT_FILES" "$lint_files"
else git ls-files -z '*.go' '*.sh' '*.md' '*Makefile' > "$lint_files"
fi

unformatted=$(xargs -0 -r sh -c '
   for file
   do case "$file" in
         *.go) gofmt -l "$file" ;;
      esac
   done
' sh < "$lint_files")
if test -n "$unformatted"
then printf 'Go files need gofmt:\n%s\n' "$unformatted" >&2
     exit 1
fi

go vet ./...
xargs -0 -r harnez lint --check < "$lint_files"
