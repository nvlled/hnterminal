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
	"net/http"
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

// I should probably use a proper API

//https://news.ycombinator.com/news?p=1
const baseUrl = "https://news.ycombinator.com"

type fetcher interface {
	fetchLinkPage(pageno int) (Toc, error)
	fetchItem(id string) (string, error)
}

type remoteFetcher struct{}

// p=0 and p=1 returns the same page
func (_ remoteFetcher) fetchLinkPage(pageno int) (Toc, error) {
	resp, err := http.Get(fmt.Sprintf("%s/news?p=%d", baseUrl, pageno))
	if err != nil {
		return nil, err
	}
	return parseLinkPage(resp.Body)
}

func (_ remoteFetcher) fetchItem(id string) (string, error) {
	filename := fmt.Sprintf("%s/item?id=%s", baseUrl, id)
	resp, err := http.Get(filename)
	if err != nil {
		return "", err
	}
	if resp.StatusCode/400 == 4 {
		return "", errors.New("Welp: :" + resp.Status)
	}
	return htmlToText(resp.Body)
}

type localFetcher struct{}

func (_ localFetcher) fetchLinkPage(pageno int) (Toc, error) {
	nap()
	filename := fmt.Sprintf("links/%d.html", pageno)
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	return parseLinkPage(file)
}

func (_ localFetcher) fetchItem(id string) (string, error) {
	nap()
	filename := fmt.Sprintf("items/%s.html", id)
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	return htmlToText(file)
}

type dirFetcher struct {
	toc   Toc
	index map[string]*TocEntry
	delay bool
}

func newDirFetcher(dir string, delayArg ...bool) *dirFetcher {
	toc, err := buildTOC(dir)
	index := make(map[string]*TocEntry)
	for _, entry := range toc {
		index[entry.ItemId] = entry
	}
	if err != nil {
		panic(err)
	}

	delay := false
	if len(delayArg) > 0 {
		delay = delayArg[0]
	}

	return &dirFetcher{
		toc:   toc,
		index: index,
		delay: delay,
	}
}

func (ft dirFetcher) fetchLinkPage(pageno int) (Toc, error) {
	if ft.delay {
		nap()
	}
	start := (pageno - 1) * viewSize
	end := min(start+viewSize, len(ft.toc))
	if start < len(ft.toc) {
		return ft.toc[start:end], nil
	}
	return nil, errors.New("no more links")
}

func (ft dirFetcher) fetchItem(id string) (string, error) {
	if ft.delay {
		nap()
	}
	entry, ok := ft.index[id]
	if !ok {
		return "", errors.New("Item not found:" + id)
	}
	file, err := os.Open(entry.SourcePath)
	if err != nil {
		return "", err
	}
	return htmlToText(file)
}

func loadToc() Toc {
	os.Mkdir(buildDir, os.ModeDir|0775)

	var toc Toc
	var err error

	filename := path.Join(buildDir, tocFilename)
	fmt.Printf("*** deserializing toc from %s\n", filename)
	toc, err = deserializeToc(filename)

	if err != nil {
		fmt.Printf("*** deserialization from %s failed, %v\n", filename, err.Error())

		fmt.Printf("*** building toc from %s\n", pageDir)
		toc, err = buildTOC(pageDir)
		if err != nil {
			panic(err)
		}

		fmt.Printf("*** serializing toc to %s\n", filename)
		err = serializeToc(toc, filename)
		if err != nil {
			panic(err)
		}
	}
	return toc
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

func savePage(entry *TocEntry) error {
	if !entry.IsExternal {
		return nil
	}

	os.Mkdir(cacheDir, os.ModeDir|0755)
	os.Mkdir(indexDir, os.ModeDir|0755)

	resp, err := http.Get(entry.Link)
	if err != nil {
		return err
	}
	filename := fmt.Sprintf("%s/%d-link.txt", cacheDir, entry.ItemId)
	destFile, err := os.Open(filename)
	if err != nil {
		return err
	}
	_, err = io.Copy(destFile, resp.Body)
	return err
}
