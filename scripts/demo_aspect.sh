#!/usr/bin/env bash
# Compare the currently supported aspect behaviors. Run from any directory.
set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."
command -v cati >/dev/null || { printf 'cati is missing from PATH; run make install first.\n' >&2; exit 1; }

printf 'Aspect comparison: Doom and vacation, rendered with 3x3 (6x3 subcells).\n'
printf 'Widths: 12 (tiny), 54 (original regression), 110 (alignment padding).\n'
printf 'Output stays in scrollback. Enter runs the next case; q quits.\n'

case_number=0
for image in assets/doom1.png assets/samples/sample-002-summer-vacation.jpg; do
    for width in 12 54 110; do
        for aspect in default aligned pixel; do
            case "$aspect" in
                default) description='Square-pixel correction for the terminal cell proportions.' ;;
                aligned) description='Integer fit when possible; continuous height. Narrow inputs use normal fit.' ;;
                pixel) description='Nearest neighbor; up to 10% alignment padding; integer aspect tweaks limited to 10% and less than one cell.' ;;
            esac
            case_number=$((case_number + 1))
            printf '\n[%d/18] %s | width %d | aspect %s\n' "$case_number" "${image##*/}" "$width" "$aspect"
            printf '%s\n' "$description"
            if [[ "$image" == assets/doom1.png && "$width" == 110 && "$aspect" == pixel ]]; then
                printf 'Doom: 640x133 content subcells; right padding 20, bottom padding 2; canvas 110x45.\n'
            fi
            args=("$image" -W "$width" -m 3x3 --aspect "$aspect")
            printf '  cati'; printf ' %q' "${args[@]}"; printf '\n'
            printf 'Press Enter to render (q to quit): '
            if ! IFS= read -r reply || [[ "$reply" == q || "$reply" == Q ]]; then
                printf '\n'
                exit 0
            fi
            cati "${args[@]}"
        done
    done
done
printf '\nAll 18 cases shown.\n'
