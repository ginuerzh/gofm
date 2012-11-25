package gofm

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	HostFM        = "http://douban.fm"
	HostMusic     = "http://music.douban.com"
	PlaylistUri   = "/j/mine/playlist"
	CaptchaUri    = "/j/new_captcha"
	CaptchaImgUri = "/misc/captcha"
	LoginUri      = "/j/login"

	ExtMp3 = ".mp3"
	ExtJpg = ".jpg"
)

const (
	// type
	BYPASS = "b" // 不再播放当前歌曲
	END    = "e" // 歌曲播放完毕
	NEW    = "n" // 更改频道，返回新列表
	LAST   = "p" // 单首歌曲播放开始且播放列表已空，请求新列表
	SKIP   = "s" // 跳过当前歌曲
	UNRATE = "u" // 取消喜欢
	RATE   = "r" // 喜欢当前歌曲

	// from
	Radio    = "radio"
	MainSite = "mainsite"
)

const (
	// channel
	CHANNEL_PRIVATE= iota + 1// 1 - 私人频道
	CHANNEL_CHN              // 2 - 华语
	CHANNEL_EU_US            // 3 - 欧美
	CHANNEL_70               // 4 - 七零
	CHANNEL_80               // 5 - 八零
	CHANNEL_90               // 6 - 九零
	CHANNEL_CANTONESE        // 7 - 粤语
	CHANNEL_ROCK             // 8 - 摇滚
	CHANNEL_FOLK             // 9 - 民谣
	CHANNEL_LIGHT            // 10 - 轻音乐
	CHANNEL_MOVIE            // 11 - 电影原声
	_                     // 12 
	_                     // 13
	CHANNEL_JAZZ             // 14 - 爵士
	CHANNEL_ELEC             // 15 - 电子
	CHANNEL_RAP              // 16 - 说唱
	CHANNEL_RB               // 17 - R&B
	CHANNEL_JP               // 18 - 日语
	CHANNEL_KOR              // 19 - 韩语
	CHANNEL_PUMA             // 20 - Puma
	CHANNEL_GIRL             // 21 - 女生
	_                     // 22
	CHANNEL_FR               // 23 - 法语
	_					  // 24
	_					  // 25
	_					  // 26
	CHANNEL_MUSICIAN         // 27 - 豆瓣音乐人
)

const (
	defaultCap = 20
	RandFloor  = 0x1000000000
	RandCeil   = 0xF000000000
)

var (
	HOME         = os.Getenv("HOME")
	USER         = os.Getenv("USER")
	TmpDir       = os.TempDir()
	gofmDir      = ".gofm/"
	songCacheDir = gofmDir + "cache/song/"
	picCacheDir  = gofmDir + "cache/pic/"
	logDir       = gofmDir
	LogFile	     = "log.txt"
	client       = &http.Client{}
)

func init() {
	var base string

	if HOME != "" {
		base = HOME
	} else if USER != "" {
		base = "/home/" + USER
	} else if TmpDir != "" {
		base = TmpDir
	} else {
		base = "/tmp"
	}

	logDir = base + "/" + logDir
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		log.Fatalln(err)
	}
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	logF, err := os.Create(logDir + LogFile)
	if err != nil {
		log.Println(err)
	}
	log.SetOutput(logF)

	songCacheDir = base + "/" + songCacheDir
	picCacheDir = base + "/" + picCacheDir
	if err := os.MkdirAll(songCacheDir, os.ModePerm); err != nil {
		log.Fatalln(err)
	}
	log.Printf("song cache: %s\n", songCacheDir)
	if err := os.MkdirAll(picCacheDir, os.ModePerm); err != nil {
		log.Fatalln(err)
	}
	log.Printf("picture cache: %s\n", picCacheDir)

}

type IntOrString interface{}

type doubanSong struct {
	Picture    string
	AlbumTitle string
	AdType     int
	Company    string
	RatingAvg  float64 `json:"rating_avg"`
	PublicTime int     `json:"public_time,string"`
	Ssid       string
	Album      string
	Like       IntOrString
	Artist     string
	Url        string
	Title      string
	SubType    string
	Length     IntOrString
	Sid        string
	Aid        string
}

func (song *doubanSong) like() bool {
	var like int
	var liked bool

	switch t := song.Like.(type) {
	case int:
		like = t
	case float64:
		like = int(t)
	case string:
		if v, err := strconv.Atoi(t); err != nil {
			like = 0
		} else {
			like = v
		}
	default:
		log.Printf("unknown type of Song.like:%T\n", t)
		like = 0
	}

	if like == 0 {
		liked = false
	} else {
		liked = true
	}

	return liked
}

func (song *doubanSong) duration() time.Duration {
	var duration time.Duration

	switch t := song.Length.(type) {
	case int:
		duration = time.Duration(t)
	case string:
		if v, err := strconv.ParseFloat(t, 64); err != nil {
			duration = time.Duration(0)
		} else {
			duration = time.Duration(v)
		}
	case float64:
		duration = time.Duration(t)
	default:
		log.Printf("unknown type of Song.Length:%T\n", t)
		duration = time.Duration(0)
	}
	return duration * time.Second
}

func (song *doubanSong) songPath() string {
	if filepath.Ext(song.Url) != ExtMp3 {
		return ""
	}
	return songCacheDir + filepath.Base(song.Url)
}

func (song *doubanSong) picPath() string {
	if filepath.Ext(song.Picture) != ExtJpg {
		return ""
	}
	return picCacheDir + filepath.Base(song.Picture)
}

func (song *doubanSong) picResource() error {
	path := song.picPath()
	if path == "" {
		return errors.New(fmt.Sprintf("resource '%s' is not picture\n", song.Picture))
	}
	data, err := Get(song.Picture)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, data, os.ModePerm)
}

func (song *doubanSong) songResource() error {
	path := song.songPath()
	if path == "" {
		return errors.New(fmt.Sprintf("resource '%s' is not song", song.Url))
	}
	data, err := Get(song.Url)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, data, os.ModePerm)
}

type Response struct {
	Result int    `json:"r"`
	Songs  []doubanSong `json:"song"`
	Error  string `json:"err"`
}

type paramList struct {
	Type     string // request type
	Channel  int
	Source   string
	PlayTime float64
	SongID   string
	Random   string
}

var defaultParam = &paramList{
	Type:    NEW,
	Channel: CHANNEL_CHN,
	Source:  MainSite,
	SongID:  "0",
}

type doubanFM struct {
	songs   []doubanSong
	current *Song
	chanSong chan *doubanSong
	channelList ChannelList
	currentChannel int
}

func NewDoubanFM() *doubanFM {
	fm := new(doubanFM)
	fm.songs = make([]doubanSong, 0, defaultCap)
	fm.current = new(Song)
	fm.chanSong = make(chan *doubanSong)
	fm.makeChannel()
	go func() {
		for song := range fm.chanSong {
			// check current song whether has been changed
			sid, _ := strconv.Atoi(song.Sid)
			if fm.current.Sid != sid {
				continue
			}
			//log.Printf("get song ID:%s, waiting... \n", song.Sid)
			if err := song.songResource(); err != nil {
				log.Println(err)
				continue
			}
			//log.Printf("data has been ready.\n")
			// check again
			if fm.current.Sid != sid {
				continue
			}
			// ok, now play
			chanPlay<- *fm.current
		}
	}()

	return fm
}

func (dfm *doubanFM) makeChannel() {
	dfm.channelList = ChannelList {
		CHANNEL_PRIVATE: "私人频道",
		CHANNEL_CHN: "华语",
		CHANNEL_EU_US: "欧美",
		CHANNEL_70: "七零",
		CHANNEL_80: "八零",
		CHANNEL_90: "九零",
		CHANNEL_CANTONESE: "粤语",
		CHANNEL_ROCK: "摇滚",
		CHANNEL_FOLK: "民谣",
		CHANNEL_LIGHT: "轻音乐",
		CHANNEL_MOVIE: "电影原声",
		CHANNEL_JAZZ: "爵士",
		CHANNEL_ELEC: "电子",
		CHANNEL_RAP: "说唱",
		CHANNEL_RB: "R&B",
		CHANNEL_JP: "日语",
		CHANNEL_KOR: "韩语",
		CHANNEL_PUMA: "Puma",
		CHANNEL_GIRL: "女生",
		CHANNEL_FR: "法语",
		CHANNEL_MUSICIAN: "豆瓣音乐人",
	}
}

// print specified channel name; if ch less than 0 print channel list.
func (dfm *doubanFM) Channel(ch int) (ChannelList, bool) {
	if ch == CHANNEL_CURRENT {
		ch = dfm.currentChannel
	}
	value, ok := dfm.channelList[ch]
	if ok {
		return ChannelList{ch:value}, ok
	}
	return dfm.channelList, ok
}

func (dfm *doubanFM) Current() *Song {
	if len(dfm.songs) == 0 {
		return dfm.current
	}
	song := &dfm.songs[0]
	dfm.current.Sid, _ = strconv.Atoi(song.Sid)
	dfm.current.Title = song.Title
	dfm.current.Album = song.AlbumTitle
	dfm.current.Artist = song.Artist
	dfm.current.Company = song.Company
	dfm.current.Public = song.PublicTime
	dfm.current.Duration = song.duration()
	dfm.current.Like = song.like()
	dfm.current.SongPath = song.songPath()
	dfm.current.PicPath = song.picPath()

	return dfm.current
}

func (dfm *doubanFM) Pre() {

}

// change channel
func (dfm *doubanFM) Tune(channel int) {
	defer dfm.check()
	//log.Printf("channel to %d\n", channel)
	defaultParam.Type = NEW
	defaultParam.Channel = channel
	defaultParam.SongID = dfm.sid()
	defaultParam.PlayTime = lastDuration.Seconds()

	songs, err := dfm.playList(defaultParam)
	if err != nil {
		log.Println(err)
		return
	}
	dfm.currentChannel = channel

	dfm.addSong(false, songs...)
	dfm.Current()
	dfm.chanSong<- &dfm.songs[0]
}

// skip to next song
func (dfm *doubanFM)Skip() {
	defer dfm.check()
	defaultParam.Type = SKIP
	defaultParam.SongID = dfm.sid()
	defaultParam.PlayTime = lastDuration.Seconds()

	songs, err := dfm.playList(defaultParam)
	if err != nil {
		log.Println(err)
		return
	}
	dfm.addSong(false, songs...)
	dfm.Current()
	dfm.chanSong<- &dfm.songs[0]
}

// report current is the last song, need more
func (dfm *doubanFM) reportLast() {
	//log.Printf("report last song\n")
	defer dfm.check()
	defaultParam.Type = LAST
	defaultParam.SongID = dfm.sid()
	defaultParam.PlayTime = lastDuration.Seconds()

	songs, err := dfm.playList(defaultParam)
	if err != nil {
		log.Println(err)
		return
	}
	dfm.addSong(true, songs...)
}

// rate current song, like or unlike
func (dfm *doubanFM) Rate(like bool) {
	defer dfm.check()
	if like {
		defaultParam.Type = RATE
	} else {
		defaultParam.Type = UNRATE
	}
	defaultParam.SongID = dfm.sid()
	defaultParam.PlayTime = lastDuration.Seconds()

	songs, err := dfm.playList(defaultParam)
	if err != nil {
		log.Println(err)
		return
	}
	dfm.addSong(true, songs...)
}

// report do not play current song any more
func (dfm *doubanFM) Trash() {
	defer dfm.check()
	defaultParam.Type = BYPASS
	defaultParam.SongID = dfm.sid()
	defaultParam.PlayTime = lastDuration.Seconds()

	songs, err := dfm.playList(defaultParam)
	if err != nil {
		log.Println(err)
		return
	}
	dfm.addSong(false, songs...)
	dfm.Current()
	dfm.chanSong<- &dfm.songs[0]
}

// reporting current song is end
func (dfm *doubanFM) Next() {
	//log.Printf("report song end\n")
	defer dfm.check()
	defaultParam.Type = END
	defaultParam.SongID = dfm.sid()
	defaultParam.PlayTime = lastDuration.Seconds()

	if _, err := dfm.playList(defaultParam); err != nil {
		log.Println(err)
		return
	}
	dfm.songs = dfm.songs[1:] // ok, next song
	dfm.Current()
	dfm.chanSong<- &dfm.songs[0]
}

func (dfm *doubanFM) check() {
	//log.Printf("playlist remain songs: %d\n", len(dfm.songs))
	dfm.Current()
	if len(dfm.songs) == 0 { // there is no song
		dfm.Tune(defaultParam.Channel)
	} else if len(dfm.songs) == 1 { // current is the last song
		dfm.reportLast()
	}
}

// add song(s) to playlist, current song reserved if keep is true
func (dfm *doubanFM) addSong(keep bool, song ...doubanSong) int {
	if len(song) == 0 {
		return 0
	}
	old := dfm.songs
	dfm.songs = make([]doubanSong, 0, len(song)+1)
	if keep {
		dfm.songs = append(dfm.songs, old[0])
	}
	dfm.songs = append(dfm.songs, song...)
	log.Printf("add %d songs\n", len(song))
	return len(song)
}

func (dfm *doubanFM) sid() string {
	if len(dfm.songs) == 0 {
		return defaultParam.SongID
	}
	return dfm.songs[0].Sid
}

// get new playlist
func (dfm *doubanFM) playList(p *paramList) ([]doubanSong, error) {
	para := fmt.Sprintf("?type=%s&channel=%d&from=%s&pt=%d&sid=%s&r=%s",
		p.Type, p.Channel - 1, p.Source, int(p.PlayTime), p.SongID, RandomStr())
	url := HostFM + PlaylistUri + para

	log.Printf("url: %s\n", url)
	data, err := Get(url)
	if err != nil {
		return nil, err
	}

	r := new(Response)
	if err = json.Unmarshal(data, r); err != nil {
		log.Println(err)
	}
	if r.Result != 0 {
		return nil, errors.New(r.Error)
	}

	return r.Songs, nil
}

func Get(url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}
	return ioutil.ReadAll(resp.Body)
}

func RandomStr() string {
	rand.Seed(int64(time.Now().Nanosecond()))
	r := rand.Int63()%RandCeil + RandFloor

	return fmt.Sprintf("%x", r)
}
