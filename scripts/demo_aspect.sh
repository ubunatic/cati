#!/usr/bin/env bash
# Compare the currently supported aspect behaviors. Run from any directory.
set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."
command -v cati >/dev/null || { printf 'cati is missing from PATH; run make install first.\n' >&2; exit 1; }

printf 'Aspect comparison: Doom and vacation, rendered with 3x3 (6x3 subcells).\n'
printf 'Doom widths: 12, 54, 107, 110, 160. Vacation widths: 12, 54, 110.\n'
printf 'Output stays in scrollback. Enter runs the next case; q quits.\n'

case_number=0
for image in assets/doom1.png assets/samples/sample-002-summer-vacation.jpg; do
    widths=(12 54 110)
    if [[ "$image" == assets/doom1.png ]]; then
        widths=(12 54 107 110 160)
    fi
    for width in "${widths[@]}"; do
        for aspect in default aligned pixel; do
            case "$aspect" in
                default) description='Square-pixel correction for the terminal cell proportions.' ;;
                aligned) description='Integer fit when possible; continuous height. Narrow inputs use normal fit.' ;;
                pixel) description='Nearest neighbor; up to 10% alignment padding; integer aspect tweaks limited to 10% and less than one cell.' ;;
            esac
            case_number=$((case_number + 1))
            if (( case_number > 1 )); then
                printf '%s\n' '------------------------'
            fi
            args=("$image" -W "$width" -m 3x3 --aspect "$aspect")
            cati "${args[@]}"
            printf '\n[%d/24] %s | width %d | aspect %s\n' "$case_number" "${image##*/}" "$width" "$aspect"
            printf '%s\n' "$description"
            if [[ "$image" == assets/doom1.png && "$width" == 110 && "$aspect" == pixel ]]; then
                printf 'Doom: 640x133 content subcells; right padding 20, bottom padding 2; canvas 110x45.\n'
            fi
            printf '  cati'; printf ' %q' "${args[@]}"; printf '\n'
            if (( case_number < 24 )); then
                printf 'Press Enter for the next case (q to quit): '
                if ! IFS= read -r reply || [[ "$reply" == q || "$reply" == Q ]]; then
                    printf '\n'
                    exit 0
                fi
                printf '\n'
            fi
        done
    done
done
printf '\nAll 24 cases shown.\n'
