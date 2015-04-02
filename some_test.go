package main

import (
	//"fmt"
	"strings"
	"testing"
)

func TestChop(t *testing.T) {
	s := `
some
lines ij asodifpjasdo asojdijfasodifjaposdi fjaposidj fpasoijdf pasodij fapsoidj fpaosidj f

asjdofiasjd fpaosidj fs
asodifja psodifj
	`
	var lines []string
	for _, line := range chopAll(s, 20) {
		line = indent(line, 0)
		lines = append(lines, line)
	}
	s = strings.Join(lines, "\n")
	println(s)

	println("--------")

	com := &Comment{
		username: "nope",
		body:     s,
		level:    1,
	}
	println(com.String())
}
