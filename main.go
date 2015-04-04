package main

import (
	"bytes"
	"code.google.com/p/go.net/html"
	"compress/gzip"
	"errors"
	"fmt"
	term "github.com/nsf/termbox-go"
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

var (
	viewSize    = 28
	buildDir    = "build"
	tocFilename = "toc"
	pageDir     = path.Join(os.Getenv("HOME"), "hnpages/new")
	cacheDir    = "cache"
	indexDir    = path.Join(cacheDir, "index")
)

//var ft = localFetcher{}

var ft = remoteFetcher{}

func main() {
	var toc Toc
	var less_ *less
	var browser *TocBrowser
	var info = infoBar{""}
	var formattedToc []string
	currentPage := 1
	tocOffset := 0

	mainLayer := createLayer(
		wind.Defer(func() wind.Layer {
			if browser == nil {
				// avoid erroneous nil comparison
				return nil
			}
			return browser.Layer()
		}),
		wind.Defer(func() wind.Layer {
			if less_ == nil {
				// avoid erroneous nil comparison
				return nil
			}
			return less_
		}),
		&info,
	)

	term.Init()
	canvas := wind.NewTermCanvas()

	draw := make(chan int, 1)
	redraw := func() { draw <- 1 }
	blocked := false
	block := func(s string) { info.contents = s; redraw(); blocked = true }
	unblock := func() { info.contents = ""; redraw(); blocked = false }

	loadItem := func() *less {
		block("fetching page...")
		redraw()

		i := browser.SelectedIndex()
		entry := toc[i]
		text, err := ft.fetchItem(entry.ItemId)
		cacheItem(entry, text)

		if err != nil {
			text = fmt.Sprintf("[%s]ERROR :: %s", entry.ItemId, err.Error())
		}
		less_ = NewLess(viewSize, text)

		unblock()
		return less_
	}
	loadLinkPage := func(n int) {
		block("fetching page...")
		currentPage += n

		newToc, _ := ft.fetchLinkPage(currentPage)
		if n > 0 {
			tocOffset += len(toc) * n
		} else {
			tocOffset += len(newToc) * n
		}

		toc = newToc
		unblock()
		formattedToc = formatToc(toc, tocOffset)
		browser = NewTocBrowser(viewSize, formattedToc)
	}
	saveItemLink := func() {
		i := browser.SelectedIndex()
		entry := toc[i]
		block("fetching [" + entry.Link + "]")
		err := savePage(entry)
		unblock()
		if err != nil {
			info.contents = "error : " + err.Error() + "; [" + entry.Link + "]"
		}
	}

	events := NewEvents()

	// everytime an event has been processed, invoke redraw()
	events.Done = func(_ term.Event) { redraw() }

	// draw thread
	go func() {
		for range draw {
			canvas.Clear()
			mainLayer.Render(canvas)
			term.Flush()
		}
	}()

	// input thread
	go func() {
		for {
			e := term.PollEvent()
			if e.Key == term.KeyCtrlC {
				term.Close()
				os.Exit(0)
			} else if e.Type == term.EventResize {
				wind.ClearCache(mainLayer)
				// tocBrowser doesn't properly resize,
				// just create a new one as a temp fix
				browser = NewTocBrowser(viewSize, formattedToc)
				redraw()
			}
			if !blocked {
				events.C <- e
			}
		}
	}()

	block("fetching page...")
	redraw()
	toc, err := ft.fetchLinkPage(currentPage)
	if err != nil {
		term.Close()
		println(err.Error())
		return
	}
	formattedToc = formatToc(toc, tocOffset)
	browser = NewTocBrowser(viewSize, formattedToc)
	unblock()
	redraw()

	events.Each(func(e term.Event) (abort bool) {
		switch e.Key {

		case term.KeyCtrlR:
			loadLinkPage(0)
		case term.KeyCtrlN:
			loadLinkPage(1)
		case term.KeyCtrlP:
			if currentPage > 1 {
				loadLinkPage(-1)
			}

		case term.KeyCtrlS:
			saveItemLink()

		// Navigation
		case term.KeyCtrlC:
			abort = true
		case term.KeyArrowDown:
			browser.SelectDown()
		case term.KeyArrowUp:
			browser.SelectUp()
		case term.KeyHome:
			browser.MoveStart()
		case term.KeyEnd:
			browser.MoveEnd()
		case term.KeyPgup:
			browser.MovePrevPage()
		case term.KeyPgdn:
			browser.MoveNextPage()

		case term.KeyEnter:
			events_ := events.Fork()
			go func() {
				for e := range events.C {
					if e.Ch == 'q' {
						close(events_.C)
						info.contents = ""
						break
					} else if e.Key == term.KeyCtrlS {
						saveItemLink()
					} else {
						events_.C <- e
					}
				}
			}()
			less_ = loadItem()
			viewText(events_, less_, loadItem)
			redraw()
			less_ = nil
		}

		return
	})
}

func createLayer(browser wind.Layer, threadView wind.Defer, info *infoBar) wind.Layer {
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
		wind.Line('─'),
		wind.SizeH(
			viewSize,
			wind.Either(
				threadView,
				browser,
			),
		),
		wind.Line('─'),
		info,
	)
}

func viewText(events *Events, less *less, refresh func() *less) {
	events.Each(func(e term.Event) (abort bool) {
		switch e.Key {
		case term.KeyHome:
			less.Home()
		case term.KeyEnd:
			less.End()
		case term.KeyPgdn:
			less.PageDown()
		case term.KeyPgup:
			less.PageUp()
		case term.KeyArrowUp:
			less.ScrollUp()
		case term.KeyArrowDown:
			less.ScrollDown()
		case term.KeyCtrlR:
			less = refresh()
		}
		return
	})
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
