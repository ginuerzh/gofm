package main

import (
	"github.com/ginuerzh/gofm"
)

func main() {
	doubanFM := gofm.NewFMPlayer("douban")
	doubanFM.Play()
}
