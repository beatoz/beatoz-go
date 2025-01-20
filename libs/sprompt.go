package libs

import (
	"bytes"
	"fmt"
	rbytes "github.com/beatoz/beatoz-go/types/bytes"
	"golang.org/x/crypto/ssh/terminal"
	"os"
	"os/signal"
)

func ClearCredential(c []byte) {
	rbytes.ClearBytes(c)
}

func ReadCredential(prompt string) []byte {
	var ret []byte
	if terminal.IsTerminal(int(os.Stdin.Fd())) {
		ret = readFromTERM(prompt, int(os.Stdin.Fd()))
	} else {
		panic("It must be executed in terminal session")
	}
	return ret
}

func readFromTERM(prompt string, fd int) []byte {
	// Get the initial state of the terminal.
	initialTermState, e1 := terminal.GetState(fd)
	if e1 != nil {
		panic(e1)
	}

	// Restore it in the event of an interrupt.
	// CITATION: Konstantin Shaposhnikov - https://groups.google.com/forum/#!topic/golang-nuts/kTVAbtee9UA
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, os.Kill)
	go func() {
		<-c
		_ = terminal.Restore(fd, initialTermState)
		os.Exit(1)
	}()

	// Now get the password.
	fmt.Print(prompt)
	p, err := terminal.ReadPassword(fd)
	fmt.Println("")
	if err != nil {
		panic(err)
	}

	// Stop looking for ^C on the channel.
	signal.Stop(c)

	// Return the password as a string.
	return bytes.TrimSpace(p)
}
