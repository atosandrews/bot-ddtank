package transport

import (
	"context"
	"io"
	"net"
	"time"

	"ddbot/relay/internal/capture"
)

type input struct {
	direction capture.Direction
	data      []byte
	err       error
	at        time.Time
}

func readStream(ctx context.Context, connection net.Conn, direction capture.Direction, output chan<- input) {
	buffer := make([]byte, 32768)

	for {
		n, err := connection.Read(buffer)
		if n == 0 && err == nil {
			err = io.ErrNoProgress
		}

		if n > 0 {
			item := input{direction: direction, data: append([]byte(nil), buffer[:n]...), at: time.Now()}

			select {
			case output <- item:
			case <-ctx.Done():
				return
			}
		}

		if err != nil {
			select {
			case output <- input{direction: direction, err: err}:
			case <-ctx.Done():
			}

			return
		}
	}
}
