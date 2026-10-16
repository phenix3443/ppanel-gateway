package main

import (
	"debug/elf"
	"debug/gosym"
	"fmt"
	"os"
	"strings"
)

func main() {
	f, err := elf.Open(os.Args[1])
	if err != nil { panic(err) }
	defer f.Close()
	text := f.Section(".text")
	pclntab := f.Section(".gopclntab")
	pcln, _ := pclntab.Data()
	lt := gosym.NewLineTable(pcln, text.Addr)
	tab, err := gosym.NewTable(nil, lt)
	if err != nil { panic(err) }
	want := os.Args[2:]
	for _, fn := range tab.Funcs {
		for _, w := range want {
			if strings.Contains(fn.Name, w) {
				fmt.Printf("0x%x\t%s\n", fn.Entry, fn.Name)
			}
		}
	}
}
