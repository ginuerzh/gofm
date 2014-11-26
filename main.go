// main
package main

import (
	//"flag"
	"log"
	//"os"
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func main() {
	p := New("douban")
	cmdLoop(p)
}

func New(name string) Player {
	switch name {
	case "douban":
		return NewDouban()
	}
	return nil
}

func cmdLoop(p Player) {
	p.Tune(1)

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Type h for help!")
	for {
		fmt.Print("gofm> ")
		cmd, _ := reader.ReadString('\n')
		cmd = strings.ToLower(strings.Trim(cmd, " \n"))

		if len(cmd) == 0 {
			continue
		}
		switch cmd[0] {
		case 'p':
			p.Play()
		case 'n':
			p.Next()
		case 'b':
			p.Prev()
		case 'x':
			p.Loop()
		case 's':
			p.Skip()
		case 't':
			p.Trash()
		case 'r':
			p.Like()
		case 'u':
			p.Unlike()
		case 'c': // current song info
			fmt.Println(p.Current())
		case 'l': // play list
			fmt.Println(p.Playlist())
		case 'z':
			p.Login()
		case 'q':
			fmt.Println("Bye!")
			os.Exit(0)
		case 'h': // help
			fallthrough
		default:
			ch, err := strconv.ParseInt(cmd, 10, 32)
			if err == nil {
				if ch > 0 && ch < CmdChannel {
					p.Tune(int(ch))
				} else {
					fmt.Println(p.Channels()) // channel list
				}
				break
			}

			help()
		}
	}
}

func help() {
	s := `Command list:
	p: 	Pause or play
	n: 	Next, next song
	b:	Prev, previous song
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
	h:	Show this help
	q:	Quit
`
	fmt.Println(s)
}
