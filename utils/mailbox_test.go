package utils_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"redwood.dev/utils"
)

func TestMailbox(t *testing.T) {
	t.Parallel()

	m := utils.NewMailbox[int](10)

	var (
		expected  = []int{2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
		toDeliver = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
		recvd     []int
	)

	for _, i := range toDeliver {
		m.Deliver(i)
	}

	chDone := make(chan struct{})
	go func() {
		defer close(chDone)
		for {
			select {
			case <-time.After(3 * time.Second):
				return
			case <-m.Notify():
				for {
					x, ok := m.Retrieve()
					if !ok {
						break
					}
					recvd = append(recvd, x)
				}
			}
		}
	}()

	<-chDone

	if len(recvd) > 10 {
		t.Fatal("received too many")
	} else if len(recvd) < 10 {
		t.Fatal("received too few")
	}
	require.Equal(t, expected, recvd)
}
