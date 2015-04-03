package main

import (
	"code.google.com/p/go.net/html"
	"fmt"
	sel "github.com/nvlled/selec"
	"strconv"
	"strings"
	"unicode"
)

const (
	INDENT_SIZE  = 40
	MAX_LINE_LEN = 90
)

type Attrs map[string]string

type Comment struct {
	username string
	body     string
	level    int
}

func parseComment(node *html.Node) Comment {
	usernameSel := []sel.Pred{
		sel.And(sel.Tag("span"), sel.Class("comhead")),
		sel.Tag("a"),
		sel.Text,
	}
	bodyNode := sel.SelectOne(node, sel.And(sel.Tag("span"), sel.Class("comment")))
	dumshit := sel.SelectOne(bodyNode, sel.And(sel.Tag("font"), sel.Attr("size", "1")))
	if dumshit != nil {
		dumshit.Parent.RemoveChild(dumshit)
	}
	var body string
	body = "(no text)"
	if bodyNode != nil {
		body = sel.TextContent(bodyNode)
	}

	imgNode := sel.SelectOne(node, sel.Tag("img"))
	indent, _ := strconv.Atoi(sel.AttrVal(imgNode, "width"))

	return Comment{
		username: getData(sel.SelectOne(node, usernameSel...)),
		body:     body,
		level:    indent / INDENT_SIZE,
	}
}

func (com Comment) String() string {
	lines := append(
		[]string{fmt.Sprintf("[%s]", com.username), ""},
		strings.Split(com.body, "\n")...,
	)

	var lines_ []string
	for _, line := range lines {
		sublines := chopAll(line, MAX_LINE_LEN)
		if len(sublines) == 0 {
			// preserve newlines
			lines_ = append(lines_, "")
		} else {
			for _, subline := range sublines {
				subline = indent(subline, com.level)
				lines_ = append(lines_, subline)
			}
		}
	}
	return strings.TrimRightFunc(strings.Join(lines_, "\n"), unicode.IsSpace) +
		"\n" + indent("──────────────────────────────", com.level)
}

type Op struct {
	title string
	link  string
	Comment
}

func (op Op) String() string {
	s := fmt.Sprintf(
		"# %s\n[%s]\n\n%s",
		op.title,
		op.Comment.username,
		op.Comment.body,
	)
	return fmt.Sprintf("# %s\n", op.link) +
		strings.Join(chopAll(s, MAX_LINE_LEN), "\n") +
		"\n────────────────────────────────────────"
}

func parseOp(node *html.Node) Op {
	titleSel := []sel.Pred{
		sel.And(sel.Tag("td"), sel.Class("title")),
		sel.Tag("a"),
	}
	usernameSel := []sel.Pred{
		sel.And(sel.Tag("td"), sel.Class("subtext")),
		sel.Tag("a"),
		sel.Text,
	}

	rows := sel.SelectAll(node, sel.Tag("tr"))
	var body string
	if len(rows) >= 3 {
		body = sel.TextContent(sel.SelectAll(rows[3], sel.Tag("td"))[1])
	}
	titleLink := sel.SelectOne(node, titleSel...)

	return Op{
		sel.TextContent(titleLink),
		strings.TrimSpace(sel.AttrVal(titleLink, "href")),
		Comment{
			username: getData(sel.SelectOne(node, usernameSel...)),
			body:     body,
		},
	}
}

func getData(node *html.Node) string {
	var data string
	if node != nil {
		data = node.Data
	}
	return strings.TrimSpace(data)
}
