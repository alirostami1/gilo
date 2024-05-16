package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"log"

	"golang.org/x/term"
)

const VERSION = "0.0.1"

const (
	CURSOR_UP = uint16(iota + 1000)
	CURSOR_DOWN
	CURSOR_LEFT
	CURSOR_RIGHT
	PAGE_UP
	PAGE_DOWN
	CTRL_C
	CTRL_L
	HOME
	END
)

type Editor struct {
	ScreenCols int
	ScreenRows int
	CursorX    int
	CursorY    int
}

var editor Editor

func main() {
	width, height, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("failed to get terminal size: %v", err)
	}

	editor = Editor{
		ScreenCols: width,
		ScreenRows: height,
		CursorX:    0,
		CursorY:    0,
	}

	prevState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("failed to switch to raw mode: %v", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), prevState)
	controlChannel := make(chan int, 0)
	redrawChannel := make(chan int, 0)

	keyChannel := make(chan uint16, 64)
	go ReadKey(keyChannel)

	go ProcessKeypress(controlChannel, keyChannel, redrawChannel)

	EditorRefreshScreen()
	go func() {
		for range redrawChannel {
			EditorRefreshScreen()
		}
	}()

	<-controlChannel
}

func CtrlKey(c byte) byte {
	return c & 0x1f
}

func ReadKey(keyChannel chan<- uint16) {
	reader := bufio.NewReaderSize(os.Stdin, 16)
	for {
		b, err := reader.ReadByte()
		if err == io.EOF {
			break
		}
		switch b {
		case CtrlKey('q'):
			keyChannel <- CTRL_C
		case CtrlKey('l'):
			keyChannel <- CTRL_L
		case '\x1b':
			b1, err := reader.ReadByte()
			if err == io.EOF {
				break
			}
			b2, err := reader.ReadByte()
			if err == io.EOF {
				break
			}

			if b1 == '[' {
				if b2 >= '0' && b2 <= '9' {
					b3, err := reader.ReadByte()
					if err == io.EOF {
						break
					}
					if b3 == '~' {
						switch b2 {
						case '1':
							keyChannel <- HOME
						case '4':
							keyChannel <- END
						case '5':
							keyChannel <- PAGE_UP
						case '6':
							keyChannel <- PAGE_DOWN
						case '7':
							keyChannel <- HOME
						case '8':
							keyChannel <- END
						}
					}
				} else {
					switch b2 {
					case 'A':
						keyChannel <- CURSOR_UP
					case 'B':
						keyChannel <- CURSOR_DOWN
					case 'C':
						keyChannel <- CURSOR_RIGHT
					case 'D':
						keyChannel <- CURSOR_LEFT
					case 'H':
						keyChannel <- HOME
					case 'F':
						keyChannel <- END
					}
				}

			} else if b1 == 'O' {
				switch b2 {
				case 'H':
					keyChannel <- HOME
				case 'F':
					keyChannel <- END
				}
			}
		}
	}
}

func ConvertKeyPressToKeyCode(keyPressChan <-chan byte, keyCodeChan chan<- int16) {
}

func ProcessKeypress(controlChannel chan<- int, keyChannel <-chan uint16, redraw chan<- int) {
	for kp := range keyChannel {
		switch kp {
		case CTRL_C:
			fmt.Print("\x1b[2J")
			fmt.Print("\x1b[H")
			controlChannel <- 0
		case CTRL_L:
			EditorRefreshScreen()
		case CURSOR_LEFT:
			if editor.CursorX > 0 {
				editor.CursorX--
			}
			redraw <- 0
		case CURSOR_DOWN:
			log.Print("hello")
			if editor.CursorY < editor.ScreenRows {
				editor.CursorY++
			}
			redraw <- 0
		case CURSOR_UP:
			if editor.CursorY > 0 {
				editor.CursorY--
			}
			redraw <- 0
		case CURSOR_RIGHT:
			if editor.CursorX < editor.ScreenCols {
				editor.CursorX++
			}
			redraw <- 0
		case PAGE_UP:
			editor.CursorY = 0
			redraw <- 0
		case PAGE_DOWN:
			editor.CursorY = editor.ScreenRows - 1
			redraw <- 0
		case HOME:
			editor.CursorX = 0
			redraw <- 0
		case END:
			editor.CursorX = editor.ScreenCols - 1
			redraw <- 0
		}
	}
}

func EditorRefreshScreen() {
	fmt.Print("\x1b[?25l\x1b[H")
	EditorDrawRows()
	fmt.Printf("\x1b[%d;%dH\x1b[?25h", editor.CursorY+1, editor.CursorX+1)
}

func EditorDrawRows() {
	builder := strings.Builder{}
	for i := 0; i < editor.ScreenRows-1; i++ {
		if i == editor.ScreenRows/3 {
			welcomeMessage := fmt.Sprintf("Gilo editor -- version %s", VERSION)
			if len(welcomeMessage) > editor.ScreenCols {
				welcomeMessage = welcomeMessage[:editor.ScreenCols-1]
			} else {
				padding := strings.Builder{}
				for j := 0; j < (editor.ScreenCols-len(welcomeMessage))/2; j++ {
					padding.WriteByte(' ')
				}
				padding.WriteString(welcomeMessage)
				padding.Write([]byte("\n\r"))
				welcomeMessage = padding.String()
			}
			builder.WriteString(welcomeMessage)
		} else {
			builder.WriteString("~ \x1b[K\r\n")
		}
	}
	builder.WriteString("~ \x1b[K")
	fmt.Print(builder.String())
}
