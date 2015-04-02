package main

import (
	"github.com/nvlled/wind"
	"github.com/nvlled/wind/size"
	"strings"
)

type less struct {
	buffer   []string
	viewSize int
	offset   int
}

func NewLess(size int, text string) *less {
	buffer := strings.Split(text, "\n")
	return &less{
		buffer:   buffer,
		viewSize: min(size, len(buffer)),
		offset:   0,
	}
}

func (less *less) Width() size.T  { return size.Free }
func (less *less) Height() size.T { return size.Const(less.viewSize) }
func (less *less) Render(canvas wind.Canvas) {
	for y, line := range less.CurrentPage() {
		canvas.DrawText(0, y, line, 0, 0)
	}
}

func (less *less) CurrentPage() []string {
	end := min(less.offset+less.viewSize, len(less.buffer))
	return less.buffer[less.offset:end]
}

func (less *less) ScrollUp() {
	if less.offset > 0 {
		less.offset--
	}
}

func (less *less) ScrollDown() {
	if less.offset < less.lastOffset() {
		less.offset++
	}
}

func (less *less) PageUp() {
	if less.offset > 0 {
		less.offset -= less.viewSize
		less.offset = max(less.offset, 0)
	}
}

func (less *less) PageDown() {
	if less.offset < less.lastOffset() {
		less.offset += less.viewSize
		less.offset = min(less.offset, less.lastOffset())
	}
}

func (less *less) Home() {
	less.offset = 0
}

func (less *less) End() {
	less.offset = less.lastOffset()
}

// rename to maxOffset
func (less *less) lastOffset() int {
	return len(less.buffer) - less.viewSize
}
