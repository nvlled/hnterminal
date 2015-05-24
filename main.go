package main

import (
	"bytes"
	"code.google.com/p/go.net/html"
	"compress/gzip"
	"errors"
	"fmt"
	term "github.com/nsf/termbox-go"
	"github.com/nvlled/control"
	sel "github.com/nvlled/selec"
	"github.com/nvlled/severe"
	"github.com/nvlled/wind"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"strings"
	"time"
)

// TODO: rename variable names

var (
	cacheDir = "cache"
	indexDir = path.Join(cacheDir, "index")
)

func createLayer(hnt *hnterminal) wind.Layer {
	return wind.Vlayer(
		wind.SetColor(
			uint16(term.ColorRed),
			uint16(term.ColorDefault),
			wind.Text(`
			┃  │ │╲  ┃
			┃──│ │ ╲ ┃ terminal
			┃  │ │  ╲┃
			`),
		),
		wind.LineH('─'),
		hnt.tab.SetElements(
			wind.Free(hnt.linkBrowser),
			hnt.threadViewer,
		),
		wind.LineH('─'),
		wind.SizeH(2, hnt.info),
	)
}

func main() {
	hnt := new(hnterminal)
	hnt.pageno = 1

	hnt.linkBrowser = newLinkBrowser(0, 0)
	hnt.threadViewer = severe.NewLess(0, 0)
	hnt.linkBrowser.AutoSize = true
	hnt.threadViewer.AutoSize = true

	hnt.tab = wind.Tab()
	hnt.info = new(infoBar)
	hnt.fetcher = remoteFetcher{}

	direct := false
	itemId := ""
	if len(os.Args) > 1 {
		arg := os.Args[1]
		if !fileExists(arg) {
			println("file not found:", arg)
		}
		if isDir(arg) {
			hnt.fetcher = newDirFetcher(arg)
		} else {
			direct = true
			itemId = arg
		}
	}

	mainLayer := createLayer(hnt)
	canvas := wind.NewTermCanvas()

	term.Init()

	drawchan := make(chan int, 1)
	draw := func() { drawchan <- 1 }
	go func() {
		draw()
		for range drawchan {
			term.Clear(0, 0)
			mainLayer.Render(canvas)
			term.Flush()
		}
	}()

	if direct {
		file, err := os.Open(itemId)

		if err != nil {
			term.Close()
			println(err.Error())
			return
		}
		text, err := htmlToText(file)
		if err != nil {
			term.Close()
			println(err.Error())
			return
		}
		hnt.threadViewer.SetText(text)
		hnt.tab.ShowIndex(1)
		draw()
		control.New(
			control.TermSource,
			control.Opts{
				EventEnded: func(_ interface{}) { draw() },
				Interrupt: control.Interrupts(
					control.KeyInterrupt(term.KeyEsc),
					control.KeyInterrupt(term.KeyCtrlC),
				),
			},
			func(flow *control.Flow) {
				hnt.controlThreadViewer(flow)
			},
		)
	} else {
		control.New(
			control.TermSource,
			control.Opts{
				EventEnded: func(_ interface{}) { draw() },
			},

			func(flow *control.Flow) {
				w, h := canvas.Dimension()
				wind.PreRender(mainLayer, w, h)
				if ft, ok := hnt.fetcher.(*dirFetcher); ok {
					_, h := hnt.linkBrowser.Size()
					ft.viewSize = h
				}

				flow.New(control.Opts{Interrupt: control.KeyInterrupt(term.KeyEsc)},
					func(flow *control.Flow) {
						hnt.loadCurrentPage(flow)
					})
				draw()

				opts := control.Opts{
					Interrupt: control.TermInterrupt(func(e term.Event, ir control.Irctrl) {
						if e.Key == term.KeyEsc {
							ir.StopNext()
						} else if e.Key == term.KeyCtrlC {
							ir.Stop()
						}
					}),
				}
				flow.TermTransfer(opts, func(flow *control.Flow, e term.Event) {
					switch e.Key {
					case term.KeyCtrlR:
						hnt.loadCurrentPage(flow)
					case term.KeyCtrlP:
						hnt.loadPrevPage(flow)
					case term.KeyCtrlN:
						hnt.loadNextPage(flow)

					case term.KeyArrowUp:
						hnt.linkBrowser.SelectUp()
					case term.KeyArrowDown:
						hnt.linkBrowser.SelectDown()

					case term.KeyEnter:
						hnt.viewSelectedThread(flow)
					}
				})
			},
		)
	}
	term.Close()
}

func formatToc(toc Toc, offset int) []string {
	var lines []string
	for i, entry := range toc {
		lines = append(lines, fmt.Sprintf("%3d. %s %s by %s [%d] [id=%s]",
			i+1+offset,
			entry.Title,
			entry.Sitebit,
			entry.Username,
			entry.NumComments,
			entry.ItemId,
		))
	}
	return lines
}

func nap() {
	time.Sleep(time.Duration(rand.Intn(2)) * time.Second)
}

func readNode(r io.Reader) (*html.Node, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(data)

	var node *html.Node
	if gfile, err := gzip.NewReader(buf); err == nil {
		node, err = html.Parse(gfile)
		gfile.Close()
	} else {
		node, err = html.Parse(buf)
	}
	return node, err
}

func htmlToText(r io.Reader) (string, error) {
	node, err := readNode(r)
	if err != nil {
		return "", err
	}

	aTable := sel.And(sel.Tag("table"), sel.AttrOnly("border", "0"))
	posts := sel.SelectAll(node, aTable)
	if len(posts) < 1 {
		return "", errors.New("no posts found, probably not a valid hn page\n")
	}

	buffer := bytes.NewBuffer(nil)
	opNode := posts[0]
	op := parseOp(opNode)
	fmt.Fprintln(buffer, op)

	if len(posts) < 3 {
		fmt.Fprintln(buffer, "(no comments)")
	} else {
		for _, post := range posts[2:] {
			fmt.Fprintln(buffer, parseComment(post))
		}
	}
	return buffer.String(), nil
}

func cacheItem(entry *TocEntry, text string) {
	os.Mkdir(cacheDir, os.ModeDir|0755)
	os.Mkdir(indexDir, os.ModeDir|0755)

	title := strings.ToLower(strings.Replace(entry.Title, " ", "-", -1))

	textname := fmt.Sprintf("%s-%s.txt", entry.ItemId, title)
	ioutil.WriteFile(path.Join(cacheDir, textname), []byte(text), 0644)

	symname := path.Join(indexDir, entry.ItemId)
	os.Symlink(path.Join("..", textname), symname)
}
