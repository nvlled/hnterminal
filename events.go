package main

import (
	term "github.com/nsf/termbox-go"
)

type Events struct {
	C    chan term.Event
	Done func(e term.Event)
}

func NewEvents() *Events {
	return &Events{
		C: make(chan term.Event, 1),
	}
}

// just copy done Handler
func (events *Events) Fork() *Events {
	return &Events{
		C:    make(chan term.Event, 1),
		Done: events.Done,
	}
}

func (events *Events) Each(fn func(e term.Event) bool) {
	for {
		e, ok := <-events.C
		if !ok {
			break
		}
		abort := fn(e)
		if events.Done != nil {
			events.Done(e)
		}
		if abort {
			break
		}
	}
}
