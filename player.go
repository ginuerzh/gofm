// player
package main

import (
	"github.com/ziutek/glib"
	"github.com/ziutek/gst"
)

const (
	CmdChannel = 100 + iota // 0 - 100 channel number
	CmdStop
	CmdPlay
	CmdPause
	CmdNext
	CmdPrev
	CmdLike
	CmdUnlike
	CmdTrash
	CmdTune
)

type Player interface {
	Stop()
	Play()
	Pause()
	Next()
	Prev()

	Like()
	Unlike()
	Trash()
	Tune(ch int)

	Channels() string
	Playlist() string
	Current() string
}

type player struct {
	cmdChan chan int
}

func (p *player) sendCmd(cmd int) {
	select {
	case p.cmdChan <- cmd:
	default:
	}
}

func (p *player) Stop() {
	p.sendCmd(CmdStop)
}

func (p *player) Play() {
	p.sendCmd(CmdPlay)
}

func (p *player) Pause() {
	p.sendCmd(CmdPause)
}

func (p *player) Next() {
	p.sendCmd(CmdNext)
}

func (p *player) Prev() {
	p.sendCmd(CmdPrev)
}

func (p *player) Like() {
	p.sendCmd(CmdLike)
}

func (p *player) Unlike() {
	p.sendCmd(CmdUnlike)
}

func (p *player) Trash() {
	p.sendCmd(CmdTrash)
}

func (p *player) Tune(ch int) {
	p.sendCmd(ch)
}

func (p *player) Playlist() string {
	return ""
}

func (p *player) Channels() string {
	return ""
}

func (p *player) Current() string {
	return ""
}

type gstreamer struct {
	mainloop *glib.MainLoop
	pipe     *gst.Element
}

func NewGstreamer() *gstreamer {
	return &gstreamer{
		mainloop: glib.NewMainLoop(nil),
		pipe:     gst.ElementFactoryMake("playbin", "mp3_pipe"),
	}
}

func (g *gstreamer) init(onMessage func(*gst.Bus, *gst.Message)) {
	bus := g.pipe.GetBus()
	bus.AddSignalWatch()
	bus.Connect("message", onMessage, nil)

	go g.mainloop.Run()

}

func (g *gstreamer) Stop() {
	g.pipe.SetState(gst.STATE_NULL)
}

func (g *gstreamer) Play() {
	g.pipe.SetState(gst.STATE_PLAYING)
}

func (g *gstreamer) Pause() {
	g.pipe.SetState(gst.STATE_PAUSED)
}

func (g *gstreamer) NewSource(uri string) {
	g.Stop()
	g.pipe.SetProperty("uri", uri)
	g.Play()
}
