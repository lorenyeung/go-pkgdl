package helpers

import (
	"fmt"
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
)

//Check logger for errors
func Check(e error, panic bool, logs string) {
	if e != nil && panic {
		log.Error(logs, " failed with error:", e)
	}
	if e != nil && !panic {
		log.Warn(logs, " failed with error:", e)
	}
}

//PrintDownloadPercent self explanatory
func PrintDownloadPercent(done chan int64, path string, total int64) {
	var stop = false
	if total == -1 {
		log.Warn("-1 Content length, can't load download bar, will download silently")
		return
	}
	for {
		select {
		case <-done:
			stop = true
		default:
			file, err := os.Open(path)
			Check(err, true, "Opening file path")
			fi, err := file.Stat()
			Check(err, true, "Getting file statistics")
			size := fi.Size()
			if size == 0 {
				size = 1
			}
			var percent = float64(size) / float64(total) * 100
			if percent != 100 {
				fmt.Printf("\r%.0f%% %s", percent, path)
			}
		}
		if stop {
			break
		}
		time.Sleep(time.Second)
	}
}
