package gofm

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"time"
	"github.com/ginuerzh/glib"
	"github.com/ginuerzh/gst"
)

type FM interface {
	Controller
	Function
}

type Controller interface {
	Pre()      // play previous song
	Skip()     // skip to next song
	PlayStop() // play or stop
	Next()     // play next song
}

type Function interface {
	Tune(channel int) // change channel
	Rate(like bool)   // rate current song, like or unlike
	Trash()           // never play current song
}

type Tag struct {
	Title    string //song title
	Album    string // album title
	Artist   string
	Company  string
	Public   int     // public time
}

type Song struct {
	Tag
	Sid      int
	Duration time.Duration// song duration time
	Like     bool
	SongPath string // song path
	PicPath  string // album picture path
}

func (song *Song) String() string {
	buffer := new(bytes.Buffer)
	info := []string{
		"\nID:" + strconv.Itoa(song.Sid),
		"\ntitle: " + song.Title,
		"\nalbum: " + song.Album,
		"\nartist: " + song.Artist,
		"\ncompany: " + song.Company,
		"\npublic time: " + strconv.Itoa(song.Public),
		"\nduration: " + song.Duration.String(),
		"\nlike: " + strconv.FormatBool(song.Like),
		"\nsong path: " + song.SongPath,
		"\npicture path: " + song.PicPath,
	}
	for i, _ := range info {
		buffer.WriteString(info[i])
	}
	return buffer.String()
}

type FMPlayer struct {
	fm       FM
	mainLoop *glib.MainLoop
	bus *gst.Bus
	pipeline *gst.Element
}

var (
	chanPlay chan Song
	lastDuration time.Duration
)

func init() {
	chanPlay = make(chan Song)
}

func NewFMPlayer(fm string) *FMPlayer {
	fmp := new(FMPlayer)
	if fm == "douban" {
		fmp.fm = NewDoubanFM()
	}

	fmp.mainLoop = glib.NewMainLoop(nil)
	fmp.pipeline = gst.ElementFactoryMake("playbin", "mp3_pipe")
	fmp.bus = fmp.pipeline.GetBus()

	fmp.bus.AddSignalWatch()
	fmp.bus.Connect("message", (*FMPlayer).onMessageError, fmp)

	go fmp.check()

	return fmp
}

func (fmp *FMPlayer) Play() {
	fmp.Tune(CHAN_MOVIE)
	fmp.mainLoop.Run()
}

func (fmp *FMPlayer) Tune(channel int) {
	go fmp.fm.Tune(channel)
}

func (fmp *FMPlayer) Skip() {
	go fmp.fm.Skip()
}

func (fmp *FMPlayer) Next() {
	go fmp.fm.Next()
}

func (fmp *FMPlayer) Rate(like bool) {
	go fmp.fm.Rate(like)
}

func (fmp *FMPlayer) Trash() {
	go fmp.fm.Trash()
}

func (fmp *FMPlayer) check() {
	for song := range chanPlay {
		fmp.pipeline.SetState(gst.STATE_NULL)
		fmp.pipeline.SetProperty("uri", "file://" + song.SongPath)
		fmp.pipeline.SetState(gst.STATE_PLAYING)
		log.Println(&song)

		cmd := exec.Command("notify-send",
					  song.Title + " - " + song.Artist,
					  song.Album + " " + strconv.Itoa(song.Public))
		if err := cmd.Run(); err != nil {
			log.Println(err)
		}
	}
}

func (fmp *FMPlayer) onMessageError(bus *gst.Bus, msg *gst.Message) {
	switch msg.GetType() {
	case gst.MESSAGE_EOS:
		var dur time.Duration
		if fmp.pipeline.QueryPosition(gst.FORMAT_TIME, &dur) {
			log.Println("play time:", dur)
			lastDuration = dur
		}
		fmp.Next()
	case gst.MESSAGE_ERROR:
		fmp.pipeline.SetState(gst.STATE_NULL)
		err, debug := msg.ParseError()
		fmt.Printf("Error: %s (debug: %s) from %s\n", err, debug, msg.GetSrc().GetName())
		fmp.mainLoop.Quit()
	}
}


