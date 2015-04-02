package main

import (
	//"fmt"
	term "github.com/nsf/termbox-go"
	"github.com/nvlled/wind"
	"github.com/nvlled/wind/size"
)

const (
	selectColor = term.ColorGreen
)

type tabLayer struct {
	sym     rune
	showSym func() bool
}

func (layer *tabLayer) Width() size.T  { return size.Const(2) }
func (layer *tabLayer) Height() size.T { return size.Const(1) }

func (layer *tabLayer) Render(canvas wind.Canvas) {
	sym := ' '
	if layer.showSym() {
		sym = layer.sym
	}
	canvas.Draw(0, 0, sym, 0, 0)
	canvas.Draw(1, 0, ' ', 0, 0)
}

type entryLayer struct {
	text       string
	isSelected func() bool
}

func (layer *entryLayer) Width() size.T  { return size.Free }
func (layer *entryLayer) Height() size.T { return size.Const(1) }

func (layer *entryLayer) Render(canvas wind.Canvas) {
	bg := term.ColorDefault
	if layer.isSelected() {
		bg = selectColor
	}
	canvas.DrawText(0, 0, layer.text, 0, uint16(bg))
}

type TocBrowser struct {
	layers     []wind.Layer
	viewHeight int
	selIndex   int
	offset     int
}

func NewTocBrowser(height int, lines []string) *TocBrowser {
	browser := &TocBrowser{
		viewHeight: min(height, len(lines)),
		selIndex:   0,
		offset:     0,
	}
	var layers []wind.Layer
	for i, line := range lines {
		index := i
		p := func() bool { return index == browser.selIndex+browser.offset }

		layers = append(layers, wind.Hlayer(
			&tabLayer{sym: '→', showSym: p},
			&entryLayer{
				text:       line,
				isSelected: p,
			},
		))
	}
	browser.layers = layers
	return browser
}

func (browser *TocBrowser) OffsetUp() bool {
	if browser.offset > 0 {
		browser.offset--
		return true
	}
	return false
}

func (browser *TocBrowser) OffsetDown() bool {
	if browser.viewHeight+browser.offset < len(browser.layers) {
		browser.offset++
		return true
	}
	return false
}

func (browser *TocBrowser) SelectUp() {
	var moved bool
	if browser.selIndex <= browser.viewHeight/2 {
		moved = browser.OffsetUp()
	}
	if !moved && browser.selIndex > 0 {
		browser.selIndex--
	}
}

func (browser *TocBrowser) SelectDown() {
	var moved bool
	if browser.selIndex >= browser.viewHeight/2 {
		moved = browser.OffsetDown()
	}
	if !moved && browser.selIndex < browser.viewHeight-1 {
		browser.selIndex++
	}
}

func (browser *TocBrowser) MoveStart() {
	if browser.offset == 0 {
		browser.selIndex = 0
	} else {
		browser.offset = 0
	}
}

func (browser *TocBrowser) MoveEnd() {
	end := len(browser.layers) - browser.viewHeight
	if browser.offset == end {
		browser.selIndex = browser.viewHeight - 1
	} else {
		browser.offset = end
	}
}

func (browser *TocBrowser) MoveNextPage() {
	browser.offset += browser.viewHeight
	browser.offset = min(browser.offset, len(browser.layers)-browser.viewHeight)
}

func (browser *TocBrowser) MovePrevPage() {
	browser.offset -= browser.viewHeight
	browser.offset = max(browser.offset, 0)
}

func (browser *TocBrowser) CurrentPage() []wind.Layer {
	// len must never be < viewHeight
	end := min(browser.offset+browser.viewHeight, len(browser.layers))
	return browser.layers[browser.offset:end]
}

func (browser *TocBrowser) SelectedIndex() int {
	return browser.offset + browser.selIndex
}

func (browser *TocBrowser) Layer() wind.Layer {
	return wind.Vlayer(browser.CurrentPage()...)
}
