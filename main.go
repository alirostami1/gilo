package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
	"log"
)

func main() {
	prevState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("failed to switch to raw mode: %v", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), prevState)

	reader := bufio.NewReader(os.Stdin)
	for {
		b, err := reader.ReadByte()
		if err == io.EOF {
			break
		}
		if b == byte('q') {
			break
		}
		if b > 31 && b < 127 {
			fmt.Printf("%d (%c)\r\n", b, b)
		} else {
			fmt.Printf("%d\r\n", b)
		}
	}
}
