package main

import (
	"code.google.com/p/go.net/html"
	"encoding/gob"
	"errors"
	sel "github.com/nvlled/selec"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
)

type TocEntry struct {
	Title       string
	Username    string
	Sitebit     string
	ItemId      string
	NumComments int
}

type Toc []*TocEntry

var titleSel = []sel.Pred{
	sel.TagAttr("td", "class", "title"),
	sel.Tag("a"),
}

var usernameSel = []sel.Pred{
	sel.TagAttr("td", "class", "subtext"),
	sel.And(sel.Tag("a"), sel.WithAttr("href", sel.HasSubstr("user?id="))),
}

var sitebitSel = []sel.Pred{
	sel.TagAttrOnly("td", "class", "title"),
	sel.And(sel.Tag("span"), sel.Class("sitebit")),
}

var itemLinkSel = []sel.Pred{
	sel.TagAttr("td", "class", "subtext"),
	sel.And(sel.Tag("a"), sel.WithAttr("href", sel.HasPrefix("item"))),
}

var comheadSel = []sel.Pred{sel.Class("comhead")}

func init() {
	gob.Register(&TocEntry{})
}

func serializeToc(toc Toc, filename string) error {
	var err error
	file, err := os.Create(filename)
	if err != nil {
		return err
	}

	enc := gob.NewEncoder(file)
	for _, entry := range toc {
		if entry.NumComments < 0 {
			continue
		}
		err = enc.Encode(*entry)
		if err != nil {
			break
		}
	}
	file.Close()
	return err
}

func deserializeToc(filename string) (Toc, error) {
	var toc Toc
	var err error

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	dec := gob.NewDecoder(file)
	for {
		var entry TocEntry
		err = dec.Decode(&entry)
		if err != nil {
			break
		}
		toc = append(toc, &entry)
	}
	if err == io.EOF {
		err = nil
	}
	return toc, err
}

func parseLinkPage(r io.Reader) (Toc, error) {
	node, err := html.Parse(r)
	if err != nil {
		return nil, err
	}
	tables := sel.SelectAll(node, sel.Tag("table"))
	if len(tables) < 3 {
		return nil, errors.New("shit")
	}
	table := tables[2]
	var toc Toc

	titleSel := []sel.Pred{
		sel.TagAttrOnly("td", "class", "title"),
		sel.Tag("a"),
	}
	ncomSel := []sel.Pred{
		sel.And(sel.Tag("td"), sel.Class("subtext")),
		sel.Last(sel.Tag("a")),
	}

	rows := sel.SelectAll(table, sel.Tag("tr"))
	for i := 0; i < len(rows); i += 3 {
		// 1st row: title, sitebit
		// 2nd row: username, numcomments, itemId
		// 3rd row: shit (skip)
		tr1 := rows[i]
		tr2 := rows[i+1]

		id := sel.AttrVal(sel.SelectOne(tr2, sel.Class("score")), "id")
		itemId := strings.TrimPrefix(id, "score_")

		numComments := func() int {
			numNode := sel.SelectOne(tr2, ncomSel...)
			content := strings.TrimSpace(sel.TextContent(numNode))
			numText := strings.TrimSuffix(content, " comments")
			n, _ := strconv.Atoi(numText)
			return n
		}()

		entry := &TocEntry{
			Title:       sel.TextContent(sel.SelectOne(tr1, titleSel...)),
			Sitebit:     sel.TextContent(sel.SelectOne(tr1, sitebitSel...)),
			Username:    sel.TextContent(sel.SelectOne(tr2, usernameSel...)),
			ItemId:      itemId,
			NumComments: numComments,
		}
		if entry.Title != "" && entry.Username != "" && numComments > 0 {
			toc = append(toc, entry)
		}
	}
	return toc, nil
}

func parseTocEntry(node *html.Node) *TocEntry {
	href := sel.AttrVal(sel.SelectOne(node, itemLinkSel...), "href")
	itemId := strings.TrimPrefix(href, "item?id=")

	entry := &TocEntry{
		Title:       selText(node, titleSel),
		Username:    selText(node, usernameSel),
		Sitebit:     selText(node, sitebitSel),
		ItemId:      itemId,
		NumComments: len(sel.SelectAll(node, comheadSel...)) - 1, // minus one to exclude OP
	}
	return entry
}

func buildTOC(dirname string) (Toc, error) {
	dir, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}

	names, err := dir.Readdirnames(-1)
	if err != nil {
		return nil, err
	}

	var toc Toc
	for _, filename := range names {
		fullname := path.Join(dirname, filename)
		if isDir(fullname) {
			continue
		}

		file, err := os.Open(fullname)
		if err != nil {
			return nil, err
		}
		node, err := html.Parse(file)
		if err != nil {
			return nil, err
		}

		entry := parseTocEntry(node)

		if entry.NumComments >= 0 {
			toc = append(toc, entry)
		}
	}
	return toc, nil
}

func isDir(filename string) bool {
	info, err := os.Lstat(filename)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func selText(node *html.Node, preds []sel.Pred) string {
	matched := sel.SelectOne(node, preds...)
	if matched != nil {
		sel.TextContent(matched)
	}
	return ""
}
