package main

import (
	"strings"
	"unicode"
)

func chop(r string, maxlen int) (string, string) {
	if len(r) < maxlen {
		return r, ""
	}
	s := r[:maxlen]
	rest := r[maxlen:]

	i := strings.LastIndexFunc(s, unicode.IsSpace)

	split := func(s string, i int) (string, string) {
		if i == len(s) {
			return s, rest
		}
		bs := []byte(s)
		// +1 to skip the whitespace
		return string(bs[0:i]), string(bs[i+1:]) + rest
	}

	if !(i > len(s)/2 && i <= maxlen) {
		i = maxlen
	}
	return split(s, i)
}

func chopAll(s string, maxlen int) []string {
	var strs []string
	var r string
	for s != "" {
		s, r = chop(s, maxlen)
		n := len(strs)
		if n > 0 && len(strs[n-1])+len(s) < maxlen {
			strs[n-1] = strs[n-1] + " " + s
		} else {
			strs = append(strs, s)
		}
		s = r
	}
	return strs
}

func collapse(s string, maxlen int) string {
	out := ""
	prevLine := ""
	for _, line := range strings.Split(s, "\n") {
		if len(prevLine)+len(line)-1 <= maxlen {
			prevLine += line + " "
		} else {
			out += prevLine + "\n"
			prevLine = line + " "
		}
	}
	return out + prevLine
}

func repeat(n int, s string) string {
	r := ""
	for i := 0; i < n; i++ {
		r += s
	}
	return r
}

func indent(s string, level int) string {
	return strings.Repeat("    ", level) + s
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}
