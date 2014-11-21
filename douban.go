// douban
package main

import (
	"bytes"
	//"container/list"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	//"strings"
	"bufio"
	"github.com/ziutek/gst"
	"os"
	"time"
)

const (
	// type
	OpBypass = "b" // 不再播放当前歌曲
	OpEnd    = "e" // 歌曲播放完毕
	OpNew    = "n" // 更改频道，返回新列表
	OpLast   = "p" // 单首歌曲播放开始且播放列表已空，请求新列表
	OpSkip   = "s" // 跳过当前歌曲
	OpUnlike = "u" // 取消喜欢
	OpLike   = "r" // 喜欢当前歌曲
)

type channel struct {
	Id     interface{} `json:"channel_id"`
	Name   string
	Intro  string
	NameEn string `json:"name_en"`
	AbbrEn string `json:"abbr_en"`
	Seq    int    `json:"seq_id"`
}

func (ch *channel) ChannelId() string {
	switch v := ch.Id.(type) {
	case int:
		return strconv.Itoa(v)
	case string:
		return v
	}
	return "0"
}

type song struct {
	Album      string
	Picture    string
	Ssid       string
	Artist     string
	Url        string
	Company    string
	Title      string
	RatingAvg  float64 `json:"rating_avg"`
	Length     int
	SubType    string
	PubTime    string `json:"public_time"`
	SongList   int    `json:"songlists_count"`
	Sid        string
	Aid        string
	Sha256     string
	Kbps       int `json:",string"`
	AlbumTitle string
	Like       int
}

func (s song) String() string {
	b := new(bytes.Buffer)
	fmt.Fprintf(b, "%10s: %s\n", "Title", s.Title)
	fmt.Fprintf(b, "%10s: %s\n", "Artist", s.Artist)
	fmt.Fprintf(b, "%10s: %s\n", "Alum", s.AlbumTitle)
	fmt.Fprintf(b, "%10s: %s\n", "Public", s.PubTime)
	fmt.Fprintf(b, "%10s: %s\n", "Company", s.Company)
	fmt.Fprintf(b, "%10s: %s\n", "Url", s.Album)
	fmt.Fprintf(b, "%10s: %f\n", "Rate", s.RatingAvg)
	fmt.Fprintf(b, "%10s: %d\n", "Like", s.Like)

	return b.String()
}

type doubanFM struct {
	player
	gst      *gstreamer
	channels []channel // channel list
	channel  int       // current channel
	paused   bool      // play status
	playlist []song
	playing  song // current playing
}

func NewDouban() Player {
	p := &doubanFM{
		paused:  true,
		channel: 2,
		gst:     NewGstreamer(),
	}
	p.channels = []channel{
		{Id: 0, Name: "私人频道", Intro: ""},
	}
	p.cmdChan = make(chan int, 1)
	p.gst.init(p.onMessage)
	go p.cmdLoop()

	return p
}

func (p *doubanFM) onMessage(bus *gst.Bus, msg *gst.Message) {
	switch msg.GetType() {
	case gst.MESSAGE_EOS:
		p.newPlaylist(OpEnd)
		p.gst.NewSource(p.next().Url)
		if len(p.playlist) == 0 {
			p.newPlaylist(OpLast)
		}
	case gst.MESSAGE_ERROR:
		//s, param := msg.GetStructure()
		//log.Println("gst error", msg.GetType(), s, param)
		p.gst.Stop()
		//err, debug := msg.ParseError()
		//log.Printf("Error: %s (debug: %s) from %s\n", err, debug, msg.GetSrc().GetName())
		//p.mainloop.Quit()
	}
}

func (p *doubanFM) cmdLoop() {
	p.getChannels()

	for {
		cmd := <-p.cmdChan
		//log.Println("cmd:", cmd)
		switch cmd {
		case CmdStop:
			p.gst.Stop()
			p.paused = true
		case CmdPlay:
			if p.paused {
				p.gst.Play()
			} else {
				p.gst.Pause()
			}
			p.paused = !p.paused
		case CmdNext:
			p.newPlaylist(OpSkip)
			p.gst.NewSource(p.next().Url)
		case CmdLike:
			p.newPlaylist(OpLike)
		case CmdUnlike:
			p.newPlaylist(OpUnlike)
		case CmdTrash:
			p.newPlaylist(OpBypass)
			p.gst.NewSource(p.next().Url)
		default:
			if cmd <= len(p.channels) {
				p.channel = cmd
				p.newPlaylist(OpNew)
				p.gst.NewSource(p.next().Url)
			}
		}
	}
}

func (p *doubanFM) next() song {
	if len(p.playlist) > 0 {
		p.playing = p.playlist[0]
		p.playlist = p.playlist[1:]
		return p.playing
	}
	return song{}
}

func (p *doubanFM) getChannels() int {
	data, err := p.get("http://douban.fm/j/app/radio/channels")
	if err != nil {
		log.Println(err)
		return 0
	}
	var r struct {
		Channels []channel `json:"channels"`
	}

	if err := json.Unmarshal(data, &r); err != nil {
		log.Println(err)
	}

	if len(r.Channels) > 0 {
		p.channels = r.Channels
	}

	return len(r.Channels)
}

func (p *doubanFM) newPlaylist(op string) int {
	v := url.Values{}
	v.Add("type", op)
	v.Add("channel", p.channels[p.channel-1].ChannelId())
	v.Add("from", "mainsite")
	v.Add("pt", strconv.Itoa(p.playing.Length))
	v.Add("sid", p.playing.Sid)

	rand.Seed(int64(time.Now().Nanosecond()))
	ra := rand.Int63()%0xF000000000 + 0x1000000000
	v.Add("r", fmt.Sprintf("%x", ra))

	data, err := p.get("http://douban.fm/j/mine/playlist?" + v.Encode())
	if err != nil {
		log.Println(err)
		return 0
	}

	var r struct {
		R          int
		QuickStart int `json:"is_show_quick_start"`
		Song       []song
		Err        string
	}

	if err := json.Unmarshal(data, &r); err != nil {
		log.Println(err)
	}

	if len(r.Song) > 0 {
		p.playlist = r.Song
	}

	return len(r.Song)
}

func (p *doubanFM) get(url string) ([]byte, error) {
	fmt.Println(url)

	r, err := p.request("GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	return ioutil.ReadAll(r.Body)
}

func (p *doubanFM) request(method, url string, body io.Reader) (*http.Response, error) {
	r, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	r.AddCookie(&http.Cookie{Name: "bid", Value: "8UK9DSCWDws"})
	if addr, err := net.ResolveTCPAddr("tcp", os.Getenv("http_proxy")); err == nil {
		conn, err := net.DialTCP("tcp", nil, addr)
		if err != nil {
			return nil, err
		}
		defer conn.Close()

		if err := r.WriteProxy(conn); err != nil {
			return nil, err
		}

		data, err := ioutil.ReadAll(conn)
		if err != nil {
			return nil, err
		}
		return http.ReadResponse(bufio.NewReader(bytes.NewBuffer(data)), r)
	}

	return http.DefaultClient.Do(r)
}

func (fm *doubanFM) Current() string {
	return fm.playing.String()
}

func (fm *doubanFM) Playlist() string {
	buffer := &bytes.Buffer{}

	if len(fm.playing.Sid) > 0 {
		fmt.Fprintf(buffer, "%s - %s (%s)\n",
			fm.playing.Title, fm.playing.Artist, fm.playing.PubTime)
	}
	for _, song := range fm.playlist {
		fmt.Fprintf(buffer, "%s - %s (%s)\n",
			song.Title, song.Artist, song.PubTime)
	}
	return buffer.String()
}

func (fm *doubanFM) Channels() string {
	buffer := &bytes.Buffer{}
	for i, ch := range fm.channels {
		fmt.Fprintf(buffer, "%2d - %s %s\n", i+1, ch.Name, ch.Intro)
	}
	return buffer.String()
}
