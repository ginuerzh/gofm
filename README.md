gofm - go douban fm
=====

用Google Go语言实现的douban.fm命令行客户端, 基本实现了douban.fm的协议(请查看API.txt)。

本应用依赖于: go1, glib-2.0, gstreamer-1.0

Go binding for glib: [github.com/ziutek/glib](http://github.com/ziutek/glib)

Go binding for gstreamer: [github.com/ziutek/gst](http://github.com/ziutek/gst)

####命令用法：
```
gofm> h
Command list:
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
```
