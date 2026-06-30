#!/usr/bin/env bash
set -euo pipefail

font="${1:-/usr/share/fonts/truetype/noto/NotoSansSymbols2-Regular.ttf}"
out="${2:-/tmp/geomshape_atlas.png}"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

files=()
for cp in $(seq 0x1FB40 0x1FB57); do
	hex=$(printf '%04X' "$cp")
	ch=$(printf '%b' "\\U$(printf '%08X' "$cp")")
	img="$tmp/$hex.png"
	convert -background white -fill black -font "$font" -pointsize 160 -gravity center label:"$ch" "$img"
	files+=("$img")
done

montage "${files[@]}" -tile 4x6 -geometry 180x180+4+4 -background white "$out"
printf '%s\n' "$out"
