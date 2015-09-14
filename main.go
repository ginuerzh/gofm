package main

import (
	"fmt"
)

func main() {
	NewDoubanFM().Run()
}

func help() {
	s := `Command list:
	p: 	Pause or play
	n: 	Next, next song
	x:	Loop, loop playback
	s:	Skip, skip current playlist
	t: 	Trash, never play
	r: 	Like
	u:	Unlike
	c:	Current playing info
	l: 	Playlist
	0: 	Channel list
	N:	Change to Channel N, N stands for channel number, see channel list
	z:	Login, Account login
	q:	Quit
	h:	Show this help
`
	fmt.Println(s)
}
