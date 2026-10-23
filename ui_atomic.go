package main

import (
	"sync/atomic"

	"github.com/egoist/mygo"
)

// atomicWindow and atomicTray hold the two long-lived UI objects. The session
// callbacks run on background goroutines and need to reach them without racing
// the main thread that creates them.

type atomicWindow struct{ value atomic.Pointer[mygo.Window] }

func (a *atomicWindow) Store(window *mygo.Window) { a.value.Store(window) }
func (a *atomicWindow) Load() *mygo.Window        { return a.value.Load() }

type atomicTray struct{ value atomic.Pointer[mygo.Tray] }

func (a *atomicTray) Store(tray *mygo.Tray) { a.value.Store(tray) }
func (a *atomicTray) Load() *mygo.Tray      { return a.value.Load() }
