package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
)

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

type dirFetcher struct {
	toc      Toc
	index    map[string]*TocEntry
	delay    bool
	viewSize int
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
		toc:      toc,
		index:    index,
		delay:    delay,
		viewSize: 28,
	}
}

func (ft dirFetcher) fetchLinkPage(pageno int) (Toc, error) {
	if ft.delay {
		nap()
	}
	start := (pageno - 1) * ft.viewSize
	end := min(start+ft.viewSize, len(ft.toc))
	if start < end {
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
