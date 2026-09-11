
## Sets
We consider the following sets with handpicked set numbers
that partially match what they represent or show the level of geometry.
The full set names use the plural form. Modenames can later be chosen freely.

**Standart Sets**

set 0:  fulls                     ␠ █
set 1:  sides*                    ␠ ▌ ▐
set 2:  halves*                   ␠ ▀ ▄
set 4:  quads*                    ␠ ▘ ▝ ▖ ▗ ▚ ▞ ▛ ▜ ▙ ▟
set 6:  sextants*                 ␠ 🬀 🬁 🬂 🬃 🬄 🬅 🬆 🬇 🬈 🬉 🬊 🬋 🬌 🬍 🬎 🬏 🬐 🬑 🬒 🬓 🬔 🬕 🬖 🬗 🬘 🬙 🬚 🬛 🬜 🬝 🬞 🬟 🬠 🬡 🬢 🬣 🬤 🬥 🬦 🬧 🬨 🬩 🬪 🬫 🬬 🬭 🬮 🬯 🬰 🬱 🬲 🬳 🬴 🬵 🬶 🬷 🬸 🬹 🬺 🬻

(*) Startted sets are not useful alone. Sets must be combined to constitute a mode.

**Bars/Spark Sets**

The following composite barchart/sparkchart sets can be used to enhance the geometry.
For better visibility, we repeat the halves and sides here in bars sets.

set 14: vbars*                    ␠ ▂ ▄ ▆
set 44: bars*                     ␠ ▂ ▄ ▆ ▎ ▌ ▊ 
set 86: morebars*                 ␠ ▁ ▂ ▃ ▄ ▅ ▆ ▇ ▎ ▍ ▌ ▋ ▊ 
set 88: allbars*                  ␠ ▁ ▂ ▃ ▄ ▅ ▆ ▇ ▏ ▎ ▍ ▌ ▋ ▊ ▉

"14" means 1x4 geometry, "41" means 4x1 geometry, "88" means 8x8 geometry.
"86" means 8x8 geometry but with fewer vertical glyphs (avoid hairlines)

**Experimental Approxitation Sets**

The following sets try to compensate gaps in the higher geometries.
Since we have no split vbars or split hbars, we need to find the best matches from other sets.

set 9:  ninelikes*               ␠ ╺ ╸ ━ ┃ ┣ ┫ ╋ ┳ ┻ ┓ ┏ ┗ ┛ ╻ ╹ ▪ ╏ ╍
set 15: vmbars*                  ␠ 🬋 🬇 🬃
set 45: mbars*                   ␠ 🬋 🬇 🬃 ┃ ╻ ╹ ▪ ╏

Ninelikes borders and the middle square set matches a 3x3 geometry and could be paired with quads or sextants.
The mbars set tries to add the missing middle bars to the vbars and bars; incl. some limited splits. Also to be paired with quads and sextants.

**More Set Ideas**

- moredots*   ␠ ┇ ┋ ┅ ┉
- diagonals*  ␠ ╱ ╲ ╳

Crazy sets will need much higher geometry levels.
Out of scope for now.

## Useful modes with shortnames and fullnames
mode f:  full     [0]                   1x1 geometry (never use as default due to bad 1:2 aspect)
mode h:  half     [0,2]                 1x2 geometry (natural 1:1 aspect with most fonts, good default with good font support)
mode q:  quad     [0,1,2,4]             2x2 geometry (bad aspect but doule the resolution)
mode Q:  quad+    [0,1,2,4,14,15]       2x4 geometry (quads withs some extra vertical splits, small extra geometry cost and some error)
mode b:  bars     [0,1,2,4,44]          4x4 geometry (4x4 from reduced v/h bars, others fit inside, lower geometry cost than full resolution bars)
mode B:  bars+    [0,1,2,4,86]          8x8 geometry (8x8 from bars; excl. hairlines, others fit inside, high geometry cost)
mode x:  six      [0,1,2,4,6]           2x6 geometry (2x2 quads and 2x3 sextants; excl. spark to keep geometry small)
mode 1:  1x1      [0]                   1x1 geometry (same as full)
mode 2:  1x2      [0,1]                 1x2 geometry (same as half)
mode 4:  2x2      [0,1,2,4]             2x2 geometry (same as quads)
mode 6:  2x3      [0,1,6]               2x3 geometry (not mixed with 2x2 geometry)
mode 9:  3x3      [0,1,6,9]             6x3 geometry (2x3 sextants and 3x3 ninelikes combined)
mode a:  all      [0,1,2,4,6,9]         6x6 geometry (2x2 quads, 2x3 sextants, 3x3 ninelikes; excl. bars to keep geometry small)
mode A:  all+     [0,1,2,4,6,9,44]      12x12 geometry (2x2 quads, 2x3 sextants, 3x3 ninelikes, 4x4 bars, excl. mbars since we have sextants already)
mode z:  z        [0,1,2,4,6,9,86]      24x24 geometry (same as 'all' but with 8x8 bars, excl. hairlines)
mode Z:  z+       [0,1,2,4,6,9,88]      24x24 geometry (same as 'z' but with full 8x8 bars, incl. hairlines)

## Debug modes (for testing)
Debug modes start with "d" and then list set numbers (not modenames)

mode d:   [0]
mode d1:  [0,1]
mode d2:  [0,2]
mode d4:  [0,4]
...
mode d1,2:  [0,1,2]
...
mode d1,2,4:  [0,1,2,4]
...
mode d1,6,9,44:  [0,1,6,9,44]

