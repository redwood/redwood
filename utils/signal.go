package utils

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brynbellomy/klog"
)

func AwaitInterrupt() <-chan struct{} {
	chDone := make(chan struct{})

	go func() {
		sigInbox := make(chan os.Signal, 1)

		signal.Notify(sigInbox, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

		count := 0
		firstTime := int64(0)

		for range sigInbox {
			count++
			curTime := time.Now().Unix()

			// Prevent un-terminated ^c character in terminal
			fmt.Println()

			if count == 1 {
				firstTime = curTime
				close(chDone)

			} else {
				if curTime > firstTime+3 {
					fmt.Println("\nReceived interrupt before graceful shutdown, terminating...")
					klog.Flush()
					os.Exit(-1)
				}
			}
		}
	}()

	return chDone
}
