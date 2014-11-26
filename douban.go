// douban
package main

import (
	"bytes"
	//"container/list"
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/ziutek/gst"
	"image/jpeg"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// type
	OpBypass = "b" // 不再播放当前歌曲并刷新列表
	OpEnd    = "e" // 歌曲播放完毕
	OpNew    = "n" // 更改频道并刷新列表
	OpLast   = "p" // 单首歌曲播放开始且播放列表已空，请求新列表
	OpSkip   = "s" // 跳过当前歌曲并刷新列表
	OpUnlike = "u" // 取消喜欢并刷新列表
	OpLike   = "r" // 喜欢当前歌曲并刷新列表
)

const (
	appName     = "radio_desktop_win"
	captchaFile = "captcha.jpg"
)

func init() {
	rand.Seed(int64(time.Now().Nanosecond()))
}

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
	album := s.Album
	if !strings.HasPrefix(album, "http") {
		album = "http://www.douban.com" + album
	}
	fmt.Fprintf(b, "%10s: %s\n", "Url", album)
	fmt.Fprintf(b, "%10s: %d\n", "Kbps", s.Kbps)
	fmt.Fprintf(b, "%10s: %f\n", "Rate", s.RatingAvg)
	fmt.Fprintf(b, "%10s: %d\n", "Like", s.Like)

	return b.String()
}

/*
type userinfo struct {
	CK     string
	Id     string
	DJ     bool `json:"is_dj"`
	Pro    bool `json:"is_pro"`
	Name   string
	Record struct {
		Banned   int
		FavChans int `json:"fav_chls_count"`
		Liked    int
		Played   int
	} `json:"play_record"`
	Uid string
	Url string
}
*/

type userinfo struct {
	Id     string `json:"user_id"`
	Name   string `json:"user_name"`
	Email  string
	Token  string
	Expire string
}

type doubanFM struct {
	player
	gst      *gstreamer
	channels []channel // channel list
	channel  int       // current channel
	paused   bool      // play status
	loop     bool
	playlist []song
	playing  song // current playing
	user     userinfo
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
		if p.loop {
			p.gst.NewSource(p.playing.Url)
		} else {
			p.newPlaylist(OpEnd)
			p.gst.NewSource(p.next().Url)
		}
		if len(p.playlist) == 0 {
			p.newPlaylist(OpLast)
		}
	case gst.MESSAGE_ERROR:
		s, param := msg.GetStructure()
		log.Println("gst error", msg.GetType(), s, param)
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
			p.gst.NewSource(p.next().Url)
			if len(p.playlist) == 0 {
				p.newPlaylist(OpLast)
			}
		case CmdLoop:
			p.loop = !p.loop
		case CmdSkip:
			p.newPlaylist(OpSkip)
			p.gst.NewSource(p.next().Url)
		case CmdLike:
			p.newPlaylist(OpLike)
			p.playing.Like = 1
		case CmdUnlike:
			p.newPlaylist(OpUnlike)
			p.playing.Like = 0
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
	resp, err := p.get("http://www.douban.com/j/app/radio/channels")
	if err != nil {
		log.Println(err)
		return 0
	}

	var r struct {
		Channels []channel `json:"channels"`
	}

	if err := json.NewDecoder(resp).Decode(&r); err != nil {
		log.Println(err)
	}

	if len(r.Channels) > 0 {
		p.channels = r.Channels
	}

	return len(r.Channels)
}

func (p *doubanFM) getLoginChls() {
	resp, err := p.get("http://douban.fm/j/explore/get_login_chls?uk=" + p.user.Id)
	if err != nil {
		return
	}
	var r struct {
		Data struct {
			Res struct {
				Favs []struct {
					Intro string
					Name  string
					Id    int
					Hot   []string `json:"hot_songs"`
				} `json:"fav_chls"`
				Recommends []struct {
					Intro string
					Name  string
					Id    int
					Hot   []string `json:"hot_songs"`
				} `json:"rec_chls"`
			}
		}
	}

	if err := json.NewDecoder(resp).Decode(&r); err != nil {
		return
	}

	for _, fav := range r.Data.Res.Favs {
		p.channels = append(p.channels,
			channel{Id: fav.Id, Name: fav.Name, Intro: fav.Intro})
	}
	for _, rec := range r.Data.Res.Recommends {
		p.channels = append(p.channels,
			channel{Id: rec.Id, Name: rec.Name, Intro: rec.Intro})
	}

}

func (p *doubanFM) newPlaylist(op string) int {
	v := url.Values{}
	v.Add("app_name", appName)
	v.Add("version", "100")
	v.Add("token", p.user.Token)
	v.Add("expire", p.user.Expire)
	v.Add("user_id", p.user.Id)
	v.Add("kbps", "192")
	v.Add("type", op)
	v.Add("channel", p.channels[p.channel-1].ChannelId())
	v.Add("sid", p.playing.Sid)
	//v.Add("from", "mainsite")
	//v.Add("pt", strconv.Itoa(p.playing.Length))
	//ra := rand.Int63()%0xF000000000 + 0x1000000000
	//v.Add("r", fmt.Sprintf("%x", ra))
	v.Add("preventCache", strconv.FormatFloat(rand.Float64(), 'f', 16, 64))

	resp, err := p.get("http://www.douban.com/j/app/radio/people?" + v.Encode())
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

	if err := json.NewDecoder(resp).Decode(&r); err != nil {
		log.Println(err)
	}

	if len(r.Song) > 0 {
		p.playlist = r.Song
	}

	return len(r.Song)
}

func (fm *doubanFM) get(url string) (io.Reader, error) {
	//fmt.Println(url)
	r, err := fm.request("GET", url, "", nil)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	b := &bytes.Buffer{}
	_, err = io.Copy(b, r.Body)

	return b, err
}

func (fm *doubanFM) post(url, bodyType string, body io.Reader) (io.Reader, error) {
	r, err := fm.request("POST", url, bodyType, body)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	b := &bytes.Buffer{}
	_, err = io.Copy(b, r.Body)

	return b, err
}

func (_ *doubanFM) request(method, url string, bodyType string, body io.Reader) (*http.Response, error) {
	r, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", bodyType)
	r.AddCookie(&http.Cookie{Name: "bid", Value: "8UK9DSCWDws"})
	proxy := os.Getenv("http_proxy")
	if len(proxy) == 0 {
		return http.DefaultClient.Do(r)
	}

	addr, err := net.ResolveTCPAddr("tcp", proxy)
	if err != nil {
		return nil, err
	}
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
		if i+1 == fm.channel {
			fmt.Fprintf(buffer, "%2d + %s %s\n", i+1, ch.Name, ch.Intro)
		} else {
			fmt.Fprintf(buffer, "%2d - %s %s\n", i+1, ch.Name, ch.Intro)
		}
	}
	return buffer.String()
}

func (fm *doubanFM) userInfo() string {
	return fmt.Sprintf("%s \t%s", fm.user.Id, fm.user.Name)
}

func (fm *doubanFM) Login() {
	var id, pwd string

	if len(fm.user.Id) > 0 {
		fmt.Println(fm.userInfo())
		return
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Douban ID: ")
		id, _ = reader.ReadString('\n')
		id = strings.Trim(id, " \t\n")
		if len(id) > 0 {
			break
		}
	}

	for {
		fmt.Print("Password: ")
		pwd, _ = reader.ReadString('\n')
		pwd = strings.TrimRight(pwd, "\n")
		if len(pwd) > 0 {
			break
		}
	}

	/*
		cid, err := fm.newCaptcha()
		if err != nil {
			log.Println(err)
			return
		}

		dir, _ := os.Getwd()
		fmt.Println("Captcha image saved at", dir+"/"+captchaFile)
		for {
			fmt.Print("Captcha: ")
			captcha, _ = reader.ReadString('\n')
			captcha = strings.Trim(captcha, " \t\n")
			if len(captcha) > 0 {
				break
			}
		}
	*/

	fm.login(id, pwd)

	if len(fm.user.Id) > 0 {
		channels := []channel{
			{Id: -3, Name: "红星兆赫", Intro: ""},
		}
		channels = append(channels, fm.channels...)
		fm.channels = channels

		fm.getLoginChls()
	}
}

func (fm *doubanFM) login(id, password string) {
	log.Println(id, password)
	formdata := &bytes.Buffer{}

	w := multipart.NewWriter(formdata)
	w.WriteField("app_name", "radio_desktop_win")
	w.WriteField("version", "100")
	w.WriteField("email", id)
	w.WriteField("password", password)
	defer w.Close()

	resp, err := fm.post("http://www.douban.com/j/app/login",
		w.FormDataContentType(), formdata)
	if err != nil {
		return
	}
	if err = json.NewDecoder(resp).Decode(&fm.user); err != nil {
		return
	}
}

func (fm *doubanFM) newCaptcha() (id string, err error) {
	r, err := fm.get("http://douban.fm/j/new_captcha")
	if err != nil {
		return
	}

	err = json.NewDecoder(r).Decode(&id)
	if err != nil {
		return
	}

	v := url.Values{}
	v.Add("size", "m")
	v.Add("id", id)
	r, err = fm.get("http://douban.fm/misc/captcha?" + v.Encode())
	if err != nil {
		return
	}

	captcha, err := jpeg.Decode(r)
	if err != nil {
		return
	}

	file, err := os.Create(captchaFile)
	if err != nil {
		return
	}
	defer file.Close()

	if err = jpeg.Encode(file, captcha, nil); err != nil {
		return
	}

	return
}
