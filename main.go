package main

import (
	"bytes"
	"code.google.com/p/go.net/html"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	term "github.com/nsf/termbox-go"
	"github.com/nvlled/control"
	sel "github.com/nvlled/selec"
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
	viewSize    = 28
	buildDir    = "build"
	tocFilename = "toc"
	pageDir     = path.Join(os.Getenv("HOME"), "hnpages/new")
	cacheDir    = "cache"
	indexDir    = path.Join(cacheDir, "index")
)

type options struct {
	local string
}

func parseFlags() (options, []string) {
	var opts options
	flag.StringVar(&opts.local, "local", "", "set local directory")
	flag.Parse()
	return opts, flag.Args()
}

func main() {

	hnt := new(hnterminal)
	hnt.pageno = 1
	hnt.linkBrowser = newLinkBrowser(-1, viewSize)
	hnt.threadViewer = NewLess(viewSize, "")
	hnt.tab = wind.Tab()
	hnt.info = new(infoBar)

	opts, _ := parseFlags()
	if opts.local != "" {
		hnt.fetcher = newDirFetcher(opts.local, true)
	} else {
		hnt.fetcher = remoteFetcher{}
	}

	mainLayer := wind.Vlayer(
		wind.SetColor(
			uint16(term.ColorRed),
			uint16(term.ColorDefault),
			wind.Text(`
			┃  │ │╲  ┃
			┃──│ │ ╲ ┃ terminal
			┃  │ │  ╲┃
			`),
		),
		wind.Line('─'),
		hnt.tab.SetElements(
			hnt.linkBrowser,
			hnt.threadViewer,
		),
		wind.Line('─'),
		hnt.info,
	)
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

	control.New(
		control.TermSource,
		control.Opts{
			EventEnded: func(_ interface{}) { draw() },
		},
		func(flow *control.Flow) {
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
					hnt.tab.ShowIndex(1)
					draw()
					hnt.viewSelectedThread(flow)
					hnt.tab.ShowIndex(0)
					draw()
				}
			})
		},
	)
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

func htmlToText(r io.Reader) (string, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return "", err
	}
	buf := bytes.NewBuffer(data)

	var node *html.Node
	if gfile, err := gzip.NewReader(buf); err == nil {
		node, err = html.Parse(gfile)
		gfile.Close()
	} else {
		node, err = html.Parse(buf)
	}

	if err != nil {
		return "", err
	}

	aTable := sel.And(sel.Tag("table"), sel.AttrOnly("border", "0"))
	posts := sel.SelectAll(node, aTable)
	if len(posts) < 1 {
		return "", errors.New("no posts found, probably not a valid hn page\n" + string(data))
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
