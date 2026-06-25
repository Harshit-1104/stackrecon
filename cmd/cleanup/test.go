package main

import (
	"fmt"
	"github.com/anknown/ahocorasick"
)

func main() {
	machine := new(goahocorasick.Machine)
	terms := [][]rune{[]rune(" test "), []rune(" phrase ")}
	err := machine.Build(terms)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	matches := machine.MultiPatternSearch([]rune(" this is a test string "), false)
	fmt.Println("Matches:", len(matches))
	for _, m := range matches {
		fmt.Println("Match word:", string(m.Word))
	}
}
