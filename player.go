package gofm

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"log"
	"os/exec"
	"os"
	"sort"
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
	Next()     // play next song
}

type Function interface {
	Tune(channel int) // change channel
	Rate(like bool)   // rate current song, like or unlike
	Trash()           // never play current song
	Channel(ch int) (ChannelList, bool)
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
		//"\nID:       " + strconv.Itoa(song.Sid),
		"title:    " + song.Title,
		"\nalbum:    " + song.Album,
		"\nartist:   " + song.Artist,
		"\ncompany:  " + song.Company,
		"\npublic :  " + strconv.Itoa(song.Public),
		//"\nduration: " + song.Duration.String(),
		"\nlike:     " + strconv.FormatBool(song.Like),
		//"\nsong path: " + song.SongPath,
		//"\npicture path: " + song.PicPath,
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
	current Song
	channel int
}

const (
	CHANNEL_CURRENT = 0
	CHANNEL_LIST    = -1
)

type ChannelList map[int]string

func (ch ChannelList) String() string {
	buffer := new(bytes.Buffer)
	keys := make([]int, len(ch))
	i := 0
	for k, _ := range ch {
		keys[i] = k
		i++
	}
	sort.Ints(keys)
	for _, k := range keys {
		buffer.WriteString(fmt.Sprintf("%2d - %s\n", k, ch[k]))
	}

	return buffer.String()
}

var (
	chanPlaylist chan Song
	lastDuration time.Duration
	pause bool
	notify bool
)

func init() {
	chanPlaylist = make(chan Song)
	notify = true
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

func (fmp *FMPlayer) Channel(ch int) (ChannelList, bool) {
	return fmp.fm.Channel(ch)
}

func (fmp *FMPlayer) Play() {
	fmp.Tune(CHANNEL_CHN)
	go fmp.mainLoop.Run()
}

func (fmp *FMPlayer) Pause() {
	if pause {
		fmp.pipeline.SetState(gst.STATE_PLAYING)
		pause = false
		fmt.Println("Playing.")
	} else {
		fmp.pipeline.SetState(gst.STATE_PAUSED)
		pause = true
		fmt.Println("Paused.")
	}
}

func (fmp *FMPlayer) Tune(channel int) {
	if ch, ok := fmp.Channel(channel); !ok {
		fmt.Println(ch)
		return
	}
	fmp.pipeline.QueryPosition(gst.FORMAT_TIME, &lastDuration)
	go fmp.fm.Tune(channel)
}

func (fmp *FMPlayer) Skip() {
	fmp.pipeline.QueryPosition(gst.FORMAT_TIME, &lastDuration)
	go fmp.fm.Skip()
}

func (fmp *FMPlayer) Next() {
	fmp.pipeline.QueryPosition(gst.FORMAT_TIME, &lastDuration)
	go fmp.fm.Next()
}

func (fmp *FMPlayer) Rate() {
	fmp.current.Like = !fmp.current.Like
	fmp.pipeline.QueryPosition(gst.FORMAT_TIME, &lastDuration)
	go fmp.fm.Rate(fmp.current.Like)
}

func (fmp *FMPlayer) Trash() {
	fmp.pipeline.QueryPosition(gst.FORMAT_TIME, &lastDuration)
	go fmp.fm.Trash()
}

func (fmp *FMPlayer) check() {
	for song := range chanPlaylist {
		log.Println("Start playing new song:", song.Sid)
		fmp.pipeline.SetState(gst.STATE_NULL)
		uri := song.SongPath
		if !strings.HasPrefix(song.SongPath, "http://") {
			uri = "file://" + song.SongPath
		}
		fmp.pipeline.SetProperty("uri", uri)
		fmp.pipeline.SetState(gst.STATE_PLAYING)
		fmp.current = song

		if notify {
			cmd := exec.Command("notify-send",
			song.Title + " - " + song.Artist,
			song.Album + " " + strconv.Itoa(song.Public))
			if err := cmd.Run(); err != nil {
				log.Println(err)
				notify = false
			}
		}
	}
}

func (fmp *FMPlayer) onMessageError(bus *gst.Bus, msg *gst.Message) {
	switch msg.GetType() {
	case gst.MESSAGE_EOS:
		fmp.Next()
	case gst.MESSAGE_ERROR:
		fmp.pipeline.SetState(gst.STATE_NULL)
		err, debug := msg.ParseError()
		log.Printf("Error: %s (debug: %s) from %s\n", err, debug, msg.GetSrc().GetName())
		fmp.mainLoop.Quit()
	}
}

func (fmp *FMPlayer) Control() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Type h for help!")
	for {
		fmt.Print("gofm> ")
		cmd, _ := reader.ReadString('\n')
		cmd = strings.ToLower(strings.Trim(cmd, " \n"))

		if len(cmd) == 0 {
			continue
		}
		if len(cmd) > 1 && strings.HasPrefix(cmd, "n") {
		}

		switch cmd[0] {
		case 'q':
			fmt.Println("Bye!")
			os.Exit(0)
		case 'p':
			fmp.Pause()
		case 's':
			fmp.Skip()
		case 'd':
			fmp.Trash()
		case 'n':
			cmd = strings.TrimLeft(cmd, "n ")
			if ch, err := strconv.Atoi(cmd); err != nil {
				list, _ := fmp.Channel(CHANNEL_LIST)
				fmt.Println(list)
			} else {
				fmp.Tune(ch)
			}
		case 'r':
			fmp.Rate()
		case 'l':
			ch, _ := fmp.Channel(CHANNEL_CURRENT)
			fmt.Println(ch)
			fmt.Println(&fmp.current)
		case 'h':
			Help()
		default:
			fmt.Println("Invalid command, type 'h' for help!")
		}
	}
}

var cmd = map[string]string {
	"h":"Help",
	"n":"Channel list",
	"nN":"Change to Channel N (N stands for channel number, see channel list)",
	"s":"Skip",
	"d":"Delete (never play)",
	"r":"Rate (like or unlike)",
	"p":"Pause or play",
	"l":"List information (current hannel and song)",
	"q":"Quit",
}

func Help() {
	buffer := new(bytes.Buffer)
	keys := make([]string, len(cmd))
	i := 0
	for k, _ := range cmd {
		keys[i] = k
		i++
	}
	sort.Strings(keys)

	fmt.Println("Command list:")
	for _, k := range keys{
		buffer.WriteString(fmt.Sprintf("%3s: %s\n", k, cmd[k]))
	}
	fmt.Println(buffer)
}
