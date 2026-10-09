# goeob

A reimplementation of *Eye of the Beholder* (1991, DOS) in Go + Ebiten.

No original game data is embedded. The engine reads your own copy's files
(`EOB.EXE`, `EOBDATA*.PAK`, `*.FNT`, …) at runtime.

    go run ./cmd/eob -data "/path/to/Eye of the Beholder.app/Contents/Resources/game"

Controls: W/S/A/D or arrows = move, Q/E = turn, Ctrl+H = help, Tab = minimap, F = attack, alternating between the two front-row characters (pad X), left click on a floor item = stow it in the emptiest backpack, Ctrl+F or F7-F10 = fullscreen, gamepad supported (see help), Space = toggle turn-based mode (extension: "TURN" badge; moving, turning, attacking, casting, throwing, wall/floor clicks and equipping cost a turn, the backpack is free), PgUp/PgDn = change level (debug).

## Status

| Area | State |
|---|---|
| PAK archives | done (`internal/pak`) |
| LCW / CPS / CMP decompression | done (`internal/res`) |
| Palette (`EOBPAL.COL`) | done |
| VCN blocks / VMP wall maps | done |
| MAZ levels | done |
| 3D dungeon view (22×15 block composer) | done |
| INF header + 0xEC/0xFB wall records | partial |
| Base wall-type table (types 0–24) from EOB.EXE | todo; fallback treats every type as a solid wall |
| Doors, decorations (`*.DAT` + decor CPS) | todo |
| Items, monsters, monster AI | todo |
| INF trigger script interpreter | todo |
| GUI: portraits, inventory, char sheet, text (`FONT6/8.FNT`) | todo |
| Character generation | todo |
| Combat, spells, rest/camp, save/load (`EOBDATA.SAV`) | todo |
| Intro/finale (`INTRO.EXE`, `*.CMP`), sound (`*.DAT` AdLib/PC speaker) | todo |

## File-format notes
- **PAK**: `{u32 offset, NUL name}*`, then the data.
- **CPS container** (`.CPS .CMP .VCN .INF`): `u16 size, u16 comp(4=LCW), u32 rawSize, u16 palSize, [pal], data`.
- **VCN**: `u16 count, 16-byte bg colour map, 16-byte alt map, count×32 bytes (8×8, 4bpp)`.
- **VMP** (raw): `u16 count, u16[330 background + 6×431 wall banks]`. Entry bits: 0x4000 = hflip, 0x8000 = overlay on block 0.
- **MAZ**: `u16 w=32, u16 h=32, u16 4`, then 1024×{N,E,S,W} wall bytes.
- **Level .PAL**: same as `EOBPAL.COL`, but the top two bits of each byte carry flags.
- **EOB.EXE**: Turbo C++ 1990 build with Borland FBOV overlays. DGROUP starts at file offset 0x23AE0.
