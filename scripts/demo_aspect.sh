#!/usr/bin/env bash
# Compare the currently supported aspect behaviors. Run from any directory.
set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."
command -v cati >/dev/null || { printf 'cati is missing from PATH; run make install first.\n' >&2; exit 1; }

printf 'Aspect comparison: Doom and vacation, rendered with 3x3 (6x3 subcells).\n'
printf 'Doom widths: 12, 54, 107, 110, 160. Vacation widths: 12, 54, 110.\n'
printf 'Up/Down (or k/j): previous/next case. Enter: next. q: quit.\n'
printf 'Output stays in scrollback, with captions below each render.\n'

case_images=()
case_widths=()
case_aspects=()
for image in assets/doom1.png assets/samples/sample-002-summer-vacation.jpg; do
    widths=(12 54 110)
    if [[ "$image" == assets/doom1.png ]]; then
        widths=(12 54 107 110 160)
    fi
    for width in "${widths[@]}"; do
        for aspect in default aligned pixel; do
            case_images+=("$image")
            case_widths+=("$width")
            case_aspects+=("$aspect")
        done
    done
done

case_count=${#case_images[@]}
case_index=0
separator=false
while true; do
    image=${case_images[case_index]}
    width=${case_widths[case_index]}
    aspect=${case_aspects[case_index]}
    case "$aspect" in
        default) description='Square-pixel correction for the terminal cell proportions.' ;;
        aligned) description='Integer fit when possible; continuous height. Narrow inputs use normal fit.' ;;
        pixel) description='Nearest neighbor; up to 10% alignment padding; integer aspect tweaks limited to 10% and less than one cell.' ;;
    esac
    if "$separator"; then
        printf '%s\n' '------------------------'
    fi
    separator=true
    args=("$image" -W "$width" -m 3x3 --aspect "$aspect")
    cati "${args[@]}"
    printf '\n[%d/%d] %s | width %d | aspect %s\n' "$((case_index + 1))" "$case_count" "${image##*/}" "$width" "$aspect"
    printf '%s\n' "$description"
    if [[ "$image" == assets/doom1.png && "$width" == 110 && "$aspect" == pixel ]]; then
        printf 'Doom: 640x133 content subcells; right padding 20, bottom padding 2; canvas 110x45.\n'
    fi
    printf '  cati'; printf ' %q' "${args[@]}"; printf '\n'
    printf 'Up/Down: previous/next | Enter: next | q: quit\n'

    while true; do
        if ! IFS= read -rsn1 key; then
            exit 0
        fi
        if [[ "$key" == $'\e' ]]; then
            # Arrow keys use either CSI (ESC [ A/B) or SS3 (ESC O A/B).
            # Bound the suffix read so a lone Escape cannot block navigation.
            suffix=''
            IFS= read -rsn2 -t 0.2 suffix || true
            key+="$suffix"
        fi
        case "$key" in
            q|Q) exit 0 ;;
            $'\e[A'|$'\eOA'|k|K)
                if (( case_index > 0 )); then
                    case_index=$((case_index - 1))
                    break
                fi
                ;;
            $'\e[B'|$'\eOB'|j|J|'')
                if (( case_index + 1 < case_count )); then
                    case_index=$((case_index + 1))
                    break
                fi
                ;;
        esac
    done
done
