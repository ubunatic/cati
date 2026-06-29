# Using more unicode goodness!

     x: 	0 	1 	2 	3 	4 	5 	6 	7 	8 	9 	A 	B 	C 	D 	E 	F
U+1FB0x 	🬀 	🬁 	🬂 	🬃 	🬄 	🬅 	🬆 	🬇 	🬈 	🬉 	🬊 	🬋 	🬌 	🬍 	🬎 	🬏
U+1FB1x 	🬐 	🬑 	🬒 	🬓 	🬔 	🬕 	🬖 	🬗 	🬘 	🬙 	🬚 	🬛 	🬜 	🬝 	🬞 	🬟
U+1FB2x 	🬠 	🬡 	🬢 	🬣 	🬤 	🬥 	🬦 	🬧 	🬨 	🬩 	🬪 	🬫 	🬬 	🬭 	🬮 	🬯
U+1FB3x 	🬰 	🬱 	🬲 	🬳 	🬴 	🬵 	🬶 	🬷 	🬸 	🬹 	🬺 	🬻 	🬼 	🬽 	🬾 	🬿
U+1FB4x 	🭀 	🭁 	🭂 	🭃 	🭄 	🭅 	🭆 	🭇 	🭈 	🭉 	🭊 	🭋 	🭌 	🭍 	🭎 	🭏
U+1FB5x 	🭐 	🭑 	🭒 	🭓 	🭔 	🭕 	🭖 	🭗 	🭘 	🭙 	🭚 	🭛 	🭜 	🭝 	🭞 	🭟
U+1FB6x 	🭠 	🭡 	🭢 	🭣 	🭤 	🭥 	🭦 	🭧 	🭨 	🭩 	🭪 	🭫 	🭬 	🭭 	🭮 	🭯

## Prep
- check which fonts support which subset
- document them here (and later in docs/)

## Research
- can we detect the font type used by the current terminal?
- check for main Linux terms

## Make it Optional
- add as new algo
- one algo for 2x3 blocks
- one for triangles and othere geoms
- one for "search for best"

## Phase 1
- start with 2x3
- add to algos
- write tests
- add to make demo-xxx cases
- add golden imgs

## Phase 2 
- add geoms algo
- repeat other phase 1 steps

## Phase 2 
- add "best" algo
- repeat other phase 1 steps

**IMPORTANT** DO NOT BREAK the other algos
**IMPORTANT** Make sure transparent row append works correctly to support 1x1 terminal <-> image asoect matching. 5x5px will always draw 1:1 in the term!
Using halfcells where needed.


