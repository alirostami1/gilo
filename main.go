package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"log"

	"golang.org/x/term"
)

const VERSION = "0.0.1"

const (
	CURSOR_UP_KEY = uint16(iota + 1000)
	CURSOR_DOWN_KEY
	CURSOR_LEFT_KEY
	CURSOR_RIGHT_KEY
	PAGE_UP_KEY
	DEL_KEY
	PAGE_DOWN_KEY
	CTRL_Q_KEY
	CTRL_L_KEY
	HOME_KEY
	END_KEY
)

type Editor struct {
	ScreenCols  int
	ScreenRows  int
	CursorX     int
	CursorY     int
	RowOffset   int
	ColOffset   int
	RenderRows  []string
	ContentRows []string
}

var editor Editor

func main() {
	flag.Parse()
	width, height, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("failed to get terminal size: %v", err)
	}

	editor = Editor{
		ScreenCols:  width,
		ScreenRows:  height,
		CursorX:     0,
		CursorY:     0,
		ContentRows: []string{},
		RowOffset:   0,
	}

	prevState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("failed to switch to raw mode: %v", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), prevState)

	args := flag.Args()
	if len(args) > 0 {
		fileName := args[0]
		err := EditorOpen(fileName)
		if err != nil {
			log.Printf("error occured while opening the file: %v", err)
			return
		}
	}

	controlChannel := make(chan int, 0)
	redrawChannel := make(chan int, 0)

	keyChannel := make(chan uint16, 64)
	go editor.ReadKey(keyChannel)

	go ProcessKeypress(controlChannel, keyChannel, redrawChannel)

	fmt.Print("\x1b[2J") // clear screen
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

func (e *Editor) ReadKey(keyChannel chan<- uint16) {
	reader := bufio.NewReaderSize(os.Stdin, 16)
	for {
		b, err := reader.ReadByte()
		if err == io.EOF {
			break
		}
		switch b {
		case CtrlKey('q'):
			keyChannel <- CTRL_Q_KEY
		case CtrlKey('l'):
			keyChannel <- CTRL_L_KEY
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
							keyChannel <- HOME_KEY
						case '3':
							keyChannel <- DEL_KEY
						case '4':
							keyChannel <- END_KEY
						case '5':
							keyChannel <- PAGE_UP_KEY
						case '6':
							keyChannel <- PAGE_DOWN_KEY
						case '7':
							keyChannel <- HOME_KEY
						case '8':
							keyChannel <- END_KEY
						}
					}
				} else {
					switch b2 {
					case 'A':
						keyChannel <- CURSOR_UP_KEY
					case 'B':
						keyChannel <- CURSOR_DOWN_KEY
					case 'C':
						keyChannel <- CURSOR_RIGHT_KEY
					case 'D':
						keyChannel <- CURSOR_LEFT_KEY
					case 'H':
						keyChannel <- HOME_KEY
					case 'F':
						keyChannel <- END_KEY
					}
				}

			} else if b1 == 'O' {
				switch b2 {
				case 'H':
					keyChannel <- HOME_KEY
				case 'F':
					keyChannel <- END_KEY
				}
			}
		}
	}
}

func ProcessKeypress(controlChannel chan<- int, keyChannel <-chan uint16, redraw chan<- int) {
	for kp := range keyChannel {
		switch kp {
		case CTRL_Q_KEY:
			fmt.Print("\x1b[2J")
			fmt.Print("\x1b[H")
			controlChannel <- 0
		case CTRL_L_KEY:
			EditorRefreshScreen()
		case CURSOR_LEFT_KEY:
			if editor.CursorX != 0 {
				editor.CursorX--
			} else if editor.CursorY > 0 {
				editor.CursorY--
				editor.CursorX = len(editor.RenderRows[editor.CursorY])
			}
			redraw <- 0
		case CURSOR_DOWN_KEY:
			if editor.CursorY < len(editor.RenderRows) {
				editor.CursorY++
			}
			redraw <- 0
		case CURSOR_UP_KEY:
			if editor.CursorY > 0 {
				editor.CursorY--
			}
			redraw <- 0
		case CURSOR_RIGHT_KEY:
			if len(editor.RenderRows) > editor.CursorY && editor.CursorX < len(editor.RenderRows[editor.CursorY]) {
				editor.CursorX++
			} else if len(editor.RenderRows) > editor.CursorY && editor.CursorX == len(editor.RenderRows[editor.CursorY]) {
				editor.CursorY++
				editor.CursorX = 0
			}
			redraw <- 0
		case PAGE_UP_KEY:
			editor.CursorY = 0
			redraw <- 0
		case PAGE_DOWN_KEY:
			editor.CursorY = editor.ScreenRows - 1
			redraw <- 0
		case HOME_KEY:
			editor.CursorX = 0
			redraw <- 0
		case END_KEY:
			editor.CursorX = editor.ScreenCols - 1
			redraw <- 0
		}
	}
}

func EditorRefreshScreen() {
	EditorScroll()
	builder := strings.Builder{}
	builder.WriteString("\x1b[?25l")
	builder.WriteString("\x1b[H")
	EditorDrawRows(&builder)
	builder.WriteString(fmt.Sprintf("\x1b[%d;%dH", editor.CursorY-editor.RowOffset+1, editor.CursorX-editor.ColOffset+1))
	builder.WriteString(fmt.Sprintf("\x1b[?25h"))

	fmt.Print(builder.String())
}

func EditorDrawRows(builder *strings.Builder) {
	for row := 0; row < editor.ScreenRows; row++ {
		fileRow := row + editor.RowOffset
		if fileRow >= len(editor.ContentRows) {
			if len(editor.ContentRows) > 0 && row == editor.ScreenRows/3 {
				welcomeMessage := fmt.Sprintf("Gilo editor -- version %s", VERSION)
				if len(welcomeMessage) > editor.ScreenCols {
					welcomeMessage = welcomeMessage[:editor.ScreenCols]
				}
				paddingLength := (editor.ScreenCols - len(welcomeMessage)) / 2
				if paddingLength > 0 {
					builder.WriteByte('~')
					paddingLength--
				}
				for paddingLength > 0 {
					builder.WriteByte(' ')
					paddingLength--
				}
				builder.WriteString(welcomeMessage)
			} else {
				builder.WriteByte('~')
			}
		} else {
			displayLength := len(editor.RenderRows[fileRow]) - editor.ColOffset
			if displayLength > 0 {
				if displayLength > editor.ScreenCols {
					displayLength = editor.ScreenCols
				}
				builder.WriteString(editor.RenderRows[fileRow][editor.ColOffset : editor.ColOffset+displayLength])
			}
		}

		builder.WriteString("\x1b[K")
		if row < editor.ScreenRows-1 {
			builder.WriteString("\r\n")
		}
		fmt.Print(builder.String())
	}
}

func EditorOpen(fileName string) error {
	file, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("failed to open the file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	numberOfLines := 0
	for scanner.Scan() {
		editor.ContentRows = append(editor.ContentRows, scanner.Text())
		editor.RenderRows = append(editor.RenderRows, RenderText(scanner.Text()))
		numberOfLines++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read file content: %w", err)
	}

	return nil
}

func EditorScroll() {
	if editor.CursorY < editor.RowOffset {
		editor.RowOffset = editor.CursorY
	}
	if editor.CursorY >= editor.RowOffset+editor.ScreenRows {
		editor.RowOffset = editor.CursorY - editor.ScreenRows + 1
	}
	if editor.CursorX < editor.ColOffset {
		editor.ColOffset = editor.CursorX
	}
	if editor.CursorX >= editor.ColOffset+editor.ScreenCols {
		editor.ColOffset = editor.CursorX - editor.ScreenCols + 1
	}
	rowLength := 0
	if editor.CursorY < len(editor.RenderRows) {
		rowLength = len(editor.RenderRows[editor.CursorY])
	}
	if editor.CursorX > rowLength {
		editor.CursorX = rowLength
	}
}

func RenderText(text string) string {
	return strings.ReplaceAll(text, "\t", "   ")
}
