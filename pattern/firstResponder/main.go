package main

import (
	"context"
	"fmt"
	"log"
	"time"
)

/*
First Responder
              ┌── Server A ── 200ms
request ──────┼── Server B ── 500ms
              └── Server C ── 100ms ← winner
*/

func query(ctx context.Context, server string, delay int) (string, error) {
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("%s context cancelled", server)
	case <-time.After(time.Duration(delay) * time.Millisecond):
		return fmt.Sprintf("%s response hello", server), nil
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	servers := []struct {
		name  string
		delay int
	}{
		{
			name:  "server-A",
			delay: 200,
		},
		{
			name:  "server-B",
			delay: 500,
		},
		{
			name:  "server-C",
			delay: 100,
		},
	}

	result := make(chan string)
	for _, server := range servers {
		go func() {
			t := time.Now()
			resp, err := query(ctx, server.name, server.delay)
			if err != nil {
				log.Print(err, " in ", time.Now().UnixMilli()-t.UnixMilli())
			}

			select {
			case <-ctx.Done():
				return
			default:
				result <- resp
			}
			cancel()
		}()
	}

	log.Print(<-result)
	close(result)
	time.Sleep(1 * time.Second)
}
