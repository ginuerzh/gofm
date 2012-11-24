package gofm

import (
	"testing"
)

var fm = NewDoubanFM()

func TestTune(t *testing.T) {
	fm.Tune(CHAN_CHN)
}

func TestReportEnd(t *testing.T) {
	for {
		if !fm.hasSong() {
			break
		}
		fm.reportEnd()
	}
}

func BenchmarkRandomStr(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if len(RandomStr()) != 10 {
			b.Fail()
		}
	}
}
