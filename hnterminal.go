package main

import (
	term "github.com/nsf/termbox-go"
	"github.com/nvlled/control"
	"github.com/nvlled/severe"
	"github.com/nvlled/wind"
)

type linkBrowser struct {
	severe.Listbox
	links   []string
	Items   severe.Items
	fetcher fetcher
}

func newLinkBrowser(w, h int) *linkBrowser {
	lb := new(linkBrowser)
	lb.Listbox = *severe.NewListbox(w, h, severe.ItemsFn(func() []string {
		return lb.links
	}))
	return lb
}

type hnterminal struct {
	linkBrowser  *linkBrowser
	threadViewer *severe.Less
	info         *infoBar

	fetcher   fetcher
	tab       wind.TabLayer
	toc       Toc
	tocOffset int
	pageno    int
}

func (hnt *hnterminal) draw(flow *control.Flow) {
	stub := 1
	flow.Send(&stub)
}

func (hnt *hnterminal) loadPage(step int, flow *control.Flow) error {
	var err error
	flow.New(control.Opts{}, func(flow *control.Flow) {
		linkBrowser := hnt.linkBrowser
		var toc Toc
		var err error

		if hnt.pageno+step <= 0 {
			return
		}

		hnt.info.contents = "loading page..."
		hnt.draw(flow)

		control.Cancellable(flow, func() {
			toc, err = hnt.fetcher.fetchLinkPage(hnt.pageno + step)
		})
		if flow.IsDead() {
			hnt.info.contents = "aborted"
			return
		}

		if err != nil {
			hnt.info.contents = "failed to load page: " + err.Error()
			return
		}

		hnt.pageno += step
		if step > 0 {
			hnt.tocOffset += len(hnt.toc) * step
		} else {
			hnt.tocOffset += len(toc) * step
		}

		linkBrowser.links = formatToc(toc, hnt.tocOffset)
		linkBrowser.RepositionCursor()
		hnt.toc = toc
		hnt.info.contents = "done loading page"
	})
	return err
}

func (hnt *hnterminal) loadCurrentPage(flow *control.Flow) {
	hnt.loadPage(0, flow)
}

func (hnt *hnterminal) loadNextPage(flow *control.Flow) {
	hnt.loadPage(+1, flow)
}

func (hnt *hnterminal) loadPrevPage(flow *control.Flow) {
	hnt.loadPage(-1, flow)
}

func (hnt *hnterminal) controlThreadViewer(flow *control.Flow) {
	flow.TermTransfer(control.Opts{}, func(flow *control.Flow, e term.Event) {
		if e.Ch == 0 {
			switch e.Key {
			case term.KeyArrowDown:
				hnt.threadViewer.ScrollDown()
			case term.KeyArrowUp:
				hnt.threadViewer.ScrollUp()
			case term.KeyArrowLeft:
				hnt.threadViewer.ScrollLeft()
			case term.KeyArrowRight:
				hnt.threadViewer.ScrollRight()
			case term.KeyCtrlB:
				fallthrough
			case term.KeyPgup:
				hnt.threadViewer.PageUp()
			case term.KeyCtrlF:
				fallthrough
			case term.KeyPgdn:
				hnt.threadViewer.PageDown()
			case term.KeyHome:
				hnt.threadViewer.ScrollStartY()
			case term.KeyEnd:
				hnt.threadViewer.ScrollEndY()
			}
		} else {
			switch e.Ch {
			case 'g':
				hnt.threadViewer.ScrollStartY()
			case 'G':
				hnt.threadViewer.ScrollEndY()
			case 'j':
				hnt.threadViewer.ScrollDown()
			case 'k':
				hnt.threadViewer.ScrollUp()
			}
		}
	})
}

func (hnt *hnterminal) viewSelectedThread(flow *control.Flow) {
	hnt.tab.ShowIndex(1)
	hnt.draw(flow)
	flow.New(control.Opts{}, func(flow *control.Flow) {
		var text string
		var err error
		var entry *TocEntry

		i, _ := hnt.linkBrowser.SelectedItem()
		if i < 0 {
			return
		}

		control.Cancellable(flow, func() {
			entry = hnt.toc[i]

			hnt.threadViewer.SetText("")
			hnt.info.contents = "loading thread data..."
			hnt.draw(flow)
			text, err = hnt.fetcher.fetchItem(entry.ItemId)

			// TODO: This should be prevented
			//       or else error: multithread access not allowed
			//flow.TermTransfer(control.Opts{}, func(flow *control.Flow, e term.Event) {
			//})
		})
		if flow.IsDead() {
			hnt.info.contents = "aborted"
			return
		}
		if err != nil {
			hnt.info.contents = "failed to load page: " + err.Error()
			return
		}
		cacheItem(entry, text)

		hnt.threadViewer.SetText(text)
		hnt.info.contents = "thread loaded"
		hnt.draw(flow)
		hnt.controlThreadViewer(flow)
	})
	hnt.tab.ShowIndex(0)
	hnt.draw(flow)
}
