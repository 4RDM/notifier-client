package main

import (
	"embed"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unsafe"

	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/gorilla/websocket"
)

var uri = url.URL{Scheme: "ws", Host: "localhost", Path: "/echo"}
//go:embed Sound.mp3
var f embed.FS

func MessageBox(hwnd uintptr, caption, title string, flags uint) int {
	ret, _, _ := syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(caption))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(title))),
		uintptr(flags))

	return int(ret)
}

func MessageBoxPlain(title, caption string) int {
	return MessageBox(0, caption, title, 0x00000030)
}

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	file, err := f.Open("Sound.mp3")

	if err != nil {
		MessageBoxPlain("Error", "Program nie mógł znaleźć pliku 'Sound.mp3'")
		os.Exit(1)
	}

	streamer, format, err := mp3.Decode(file)
	if err != nil {
		MessageBoxPlain("Error", "Program nie przetworzyć pliku dźwiękowego")
		os.Exit(1)
	}

	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

	c, _, err := websocket.DefaultDialer.Dial(uri.String(), nil)
	if err != nil {
		MessageBoxPlain("Error", "Program nie mógł połączyć się z serwerem")
		os.Exit(1)
	}
	defer c.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				return
			}

			if string(message) == "echo" {
				streamer.Seek(0)
				speaker.Play(streamer)
			}
		}
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case t := <-ticker.C:
			err := c.WriteMessage(websocket.TextMessage, []byte(t.String()))
			if err != nil {
				return
			}
		case <-interrupt:
			c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			select {
			case <-done:
			case <-time.After(time.Second * 3):
			}
			return
		}
	}
}
