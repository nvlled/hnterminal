package main

import (
	"github.com/nvlled/wind"
	"github.com/nvlled/wind/size"
)

type infoBar struct {
	contents string
}

func (info *infoBar) Width() size.T  { return size.Free }
func (info *infoBar) Height() size.T { return size.Free }
func (info *infoBar) Render(canvas wind.Canvas) {
	canvas.DrawText(0, 0, info.contents, 0, 0)
}
