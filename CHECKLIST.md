# Port checklist

Legend: `[x]` done · `[~]` partially done · `[ ]` not started.
All game assets, tables and text are read from the original files at runtime (EOB.EXE, *.PAK, *.FNT).

## 0. Infrastructure
- [x] PAK archive reader
- [x] LCW / CPS / CMP decompression
- [x] Palette loading (EOBPAL.COL; level .PAL files)
- [x] Disassembly toolkit (tools/: listing, symbols, xrefs, function index, overlays resolved)
- [x] EXE data access package (DGROUP tables + strings read from EOB.EXE at runtime)
- [x] Engine runs in its own goroutine with a System interface (present, input, timing)
- [x] Headless screenshot/test harness (scripted input → PNG)
- [x] Screen: pages, screen dims, rect copy/fill, shape blit (flip, remap, clip), scaling
- [x] Fonts (FONT6.FNT / FONT8.FNT) and text printing / text windows
- [x] Palette fades and timing (ticks) (EOB.EXE sets the VGA palette once, no in-game fades; intro/finale fades belong to section 8)

## 1. Level loading
- [x] MAZ wall grid
- [x] VCN blocks / VMP wall banks
- [x] INF header parsing (names, door types, script timers, steps until script call, level .PAL, monster timer reset)
- [x] Default wall tables (vmp/shape/special/flags) read from EXE
- [x] 0xEC decoration sets (.DAT + CPS) and 0xFB wall assignments
- [x] Monster sheets (encode tables), monster timers, 30 monster placements
- [x] Block flags + script offsets section
- [x] Item placement linked lists (ITEM.DAT; keyboard-mode pickup S0acd_0ba6/S0bd2_0458 ported; keyboard dispatcher S0acd_0d9d with 0a34/091c/09ef/0a61/08e7 ported)
- [x] Door shapes (DOOR.CPS + encode tables), door switches
- [x] Level state persistence between levels (LEVELS.TMP equivalent)

## 2. Scene rendering
- [x] Block drawing buffer (walls/floor/ceiling via VMP)
- [x] Exact flip/colour-table handling (wllProcessFlag), fixed view at correct screen position
- [x] Decorations (drawDecorations)
- [x] Doors and door switches (per-level door styles)
- [x] Items on floor and in wall niches (drawBlockItems)
- [x] Monsters, scaled, flipped, hit-flash remap (drawMonsters)
- [x] Thrown items / spell missiles (flying objects)
- [x] Special walls (e.g. wall type 0x34 effect)
- [x] Compass
- [x] Scene clipping (setLevelShapesDim, dscDim tables)

## 3. Items & data
- [x] ITEM.DAT items + names, ITEMTYPE.DAT
- [x] Item graphics (ITEMICN, ITEML1, ITEMS1, THROWN, item remap)
- [x] Item names (identified / unidentified, +N, cursed) (S0bd2_05b5, printed on pickup)

## 4. GUI / playfield
- [x] Playfield layout (PLAYFLD.CPS), character portrait boxes
- [x] Portraits, names, HP (bar graphs / numbers), hands, status overlays (S1547_0991 all modes, 028c/033f/04a0/0544/0642/07e7; keyboard-slot highlight ds:f51a never set)
- [x] Text window: S1de7_060a/03b2/0009/00b6/0344/02fe (control codes 02/06/09/0d/13, 0x80 end, pixel word wrap, scroll); overflow hook [0x8117] = O211e MORE (seqMore, wired); 00c9/025e/05df uncalled (exempt)
- [x] Mouse cursor with held item
- [x] Movement / turn buttons, keyboard controls
- [x] Inventory screen (INVENT.CPS), equip rules, food, eating
- [x] Character info / stats screen (S0acd_018e toggle + stats button lists, visually checked)
- [x] Hand attacks, results text (MISS/HACK/…)
- [x] Party reordering (swapping) (S0acd_0264/0393, blink timer 0)
- [x] Clicking on walls (switches, keyholes, niches), picking up/dropping/throwing items

## 5. Game logic
- [x] Party movement, collisions, turning, steps counter
- [x] Doors: open/close/animate, force door, pry, levers
- [x] Level script interpreter (INF opcodes) + triggers (all 26 opcodes + eval; helpers ported)
- [x] Timers (character, monster, script timers) (all 16 ds:5ba8 slots have ported callbacks: 0000 swap blink, 0285 flying, 05df monsters, 07ca chars, 03b4 door, 04b0 special wall, 0c82 food, 0ce7 jitter; S08e6_1410 script timers/respawn)
- [x] Monsters: AI, movement, attacks, ranged attacks, death, drops
- [x] Combat: to-hit, damage, thrown weapons, ammunition
- [x] Automatic turn undead on levels 2 and 7 (S0d1c_0a43, 0903, 09cb, 08b1)
- [x] Character conditions: poison, paralysis, death, unconscious, food (timers ported)
- [x] Experience & levelling
- [x] Teleporters, pits, stairs, level changes (script ops 0xFA/0xEC, moveParty, leave-level cleanup O210c_19f7; round-trip test)
- [x] NPC encounters / joining party, dialogues (O211e: all 8 NPC sequences, TEXT.DAT dialogue, choice buttons, MORE, join/drop/resurrect/heal; sequences.go)
- [x] Special events and sequences (portal O211e_1204 with dissolve, steal-item opcode 0xf9; id 10 = copy protection, skipped)

## 6. Magic
- [x] Mage & cleric spell books, memorize / pray (spellui.go S0ef3_21e0-2929; camp.go O2102_00dd/025d/0881/097a; rest.go O212a)
- [x] Spell casting UI (spellbook menu, tabs, rows, abort, target selection)
- [x] All EOB1 spell effects (all 53 spell callbacks ported; explosion S0ef3_00cd/2a53, cone S0ef3_18d8/2ebc, portrait sparkle S0ef3_0665, Silvias sparkle S0ef3_06e7 in spellfx.go)
- [x] Wands / scrolls / potions (useitem.go; wand of Silvias visual simplified; readable TEXT.DAT items (type 0xb) via O211e_1cdd)

## 7. Camp menu
- [x] Rest, memorize, pray, scribe scrolls (camp.go, rest.go: O212a rest with spell learning, healer spells, 8h HP/lay-on-hands/starvation, poison damage, monster check, +0x7ff8 ticks/hour; char highlight S0d1c_0454 in select-char and memorize)
- [x] Preferences (tunes, sounds, bar graphs, mouse) (tunes/sounds only toggle the flags)
- [x] Save / load game (EOBDATA.SAV) (SaveGame mirrors LoadOriginalSave)
- [x] Drop character, quit

## 8. Front end
- [x] Intro sequence (intro.go: INTRO.EXE overlay O0e96 ported 1:1, tables/filenames read from INTRO.EXE; dissolve/line-wipe pacing approximated; `shot -intro MS` renders frames)
- [x] Main menu (menu.go: O2106 title+menu, S14ef generic menus; load/new party/exit; new game = level 1, block 0x1ea)
- [x] Character generation (chargen.go, O20f6: slots, race/class/alignment, 4d6 stats, portraits+blink, sheet, view/delete, starting items, Modify stat editor O20f6_1add/135d with HP/AC recompute, name editor S082c_000b (insert, cursor, backspace, delete, Esc), keyboard highlight S0d1c_0454, FONT8 as chargen font. Cursor blink is static like the original's redraw)
- [x] Font switching S1a47_0169 (setFont): FONT8 at init/main menu/chargen/camp menus, messages, confirms, rest hours; FONT6 at game start, save load, char panels (S1547_0991), select-char loop, rest loop, monsters-near check, NPC sequences. Not ported: callers in unported S1c0e_0add (disk prompt) and O2106_09ed (copy protection)
- [x] Copy protection: intentionally not ported (skipped entirely, per user request)
- [x] Finale / death screens (Xanathar death XDEATH1-3, ending text + Beholder Bonus password; party death = original text box + exit; sound calls are stubs)

## 9. Sound
- [x] OPL2 FM emulation (internal/opl: integer YM3812 model — log-sin/exp ROMs, 9-bit EG with global rate counter, phase accumulators, tremolo/vibrato LFOs, 4 waveforms, feedback; register writes applied one per native sample like the real chip's write delay; linear resampling. Rhythm mode not emulated: no game data enables it)
- [x] AdLib sound effects: internal/adlib ports the Westwood v1 driver, plays ADLIB.DAT via playSound (S177d_0070, gated by Sounds flag 0x80e6). playSoundAt/playSoundUpdate port S177d_0106/00f3 (one-deep queue flushed each main-loop pass; AdLib path ignores distance, as the original does; digital-path distance volume not ported). Driver frequency and pitch-bend tables are read from the driver embedded in EOB.EXE / INTRO.EXE (located structurally), setupNote/0xb9 ported exactly; 0xc3 (rhythm off) ported; the other rhythm opcodes are unused by ADLIB.DAT and SOUND.DAT. Main-loop ambient sound 0x1e ported (ambientSound, levels 1-3).
- [x] Music: EOB.EXE plays none (Tunes flag 0x80e8 only toggled/saved). SOUND.DAT (INTRO.EXE music, same format) plays via MusicPlayer/playMusic, driven by the intro port.
- [ ] PC speaker fallback (optional)

## 10. Verification
- [x] Play through level 1 to level 2 (TestPlaythroughLevel1: headless bot through Run — levers, door buttons, pressure plates, fights, stairs)
- [ ] Full playthrough to the ending
- [x] Level sweep: every enterable block of levels 1-12 entered through Run (TestLevelSweep; answers NPC dialogue, returns from stairs to a script-free block)
- [x] Routine inventory: tools/parity.py exits 0 — all 749 EOB.EXE routines ported (520) or exempt with reason (229)
- [x] Ghidra parity review of every ported routine: tools/review_cov.py exits 0 — 520/520 reviewed in tools/parity_review.txt (match/fixed/gap). Known gaps (documented, original-memory quirks): O20f6_26c6 class item -1 table tail, O20f6_00f7 low-memory flag, O211e_1e1b dissolve pacing.

## 11. Extensions (not in the original)
- [x] Turn-based mode (Space, "TURN" badge; moves, attacks, casts, throws, wall/floor clicks, equipping cost a turn)
- [x] Minimap (Tab), help overlay (Ctrl+H, pauses), fullscreen (Ctrl+F / F7-F10), hidden OS cursor
- [x] Gamepad (standard layout: stick cursor, A/B click, D-pad move, LB/RB strafe, X turn mode, Y map, Start camp, Back help)
- [x] Letter keys mapped to the original key numbers (C camp, I inventory, P info, U/T/G/M keyboard mode, 1-5 spell levels)
- [x] Quick start main-menu entry: preset balanced party (paladin, fighter, cleric, fighter/mage), best of 30 rolls
