package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/felix/goeob/internal/eob"
)

func main() {
	dir := os.Args[1]
	e, err := eob.New(dir, nil)
	if err != nil {
		panic(err)
	}
	if err := e.DebugLoadSave(filepath.Join(dir, "EOBDATA.SAV")); err != nil {
		panic(err)
	}
	b := e.CurrentBlock
	fmt.Println("block", b, "dir", e.CurrentDirection, "head", e.Blocks[b].ItemHead)
	h := int(e.Blocks[b].ItemHead)
	if h != 0 {
		si := h
		for {
			it := e.Items[si]
			fmt.Printf("item %d nameid %d icon %d type %d pos %d next %d prev %d %q\n", si, it.NameID, it.Icon, it.Type, it.Pos, it.Next, it.Prev, e.ItemNames[it.NameID])
			si = int(it.Prev)
			if si == h {
				break
			}
		}
	}
}

func init() {
	if len(os.Args) > 2 && os.Args[2] == "monsters" {
		e, _ := eob.New(os.Args[1], nil)
		e.DebugLoadSave(filepath.Join(os.Args[1], "EOBDATA.SAV"))
		for i, m := range e.Monsters {
			if m.Block != 0 {
				fmt.Printf("m%d type %d unit %d block %d (%d,%d) pos %d dir %d mode %d hp %d\n", i, m.Type, m.Unit, m.Block, m.Block%32, m.Block/32, m.Pos, m.Dir, m.Mode, m.HPCur)
			}
		}
		os.Exit(0)
	}
}
