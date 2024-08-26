package libs

import (
	"bufio"
	"bytes"
	"fmt"
	rbytes "github.com/beatoz/beatoz-go/types/bytes"
	"golang.org/x/crypto/ssh/terminal"
	"os"
	"os/signal"
	"syscall"
)

func ClearCredential(c []byte) {
	rbytes.ClearBytes(c)
}

func ReadCredential(prompt string) []byte {
	var ret []byte
	if terminal.IsTerminal(int(os.Stdin.Fd())) {
		ret = readFromTERM(prompt)
	} else {
		ret = readFromSTTY(prompt)
	}
	return ret
}

func readFromTERM(prompt string) []byte {
	// Get the initial state of the terminal.
	initialTermState, e1 := terminal.GetState(int(syscall.Stdin))
	if e1 != nil {
		panic(e1)
	}

	// Restore it in the event of an interrupt.
	// CITATION: Konstantin Shaposhnikov - https://groups.google.com/forum/#!topic/golang-nuts/kTVAbtee9UA
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, os.Kill)
	go func() {
		<-c
		_ = terminal.Restore(int(syscall.Stdin), initialTermState)
		os.Exit(1)
	}()

	// Now get the password.
	fmt.Print(prompt)
	p, err := terminal.ReadPassword(int(syscall.Stdin))
	fmt.Println("")
	if err != nil {
		panic(err)
	}

	// Stop looking for ^C on the channel.
	signal.Stop(c)

	// Return the password as a string.
	return bytes.TrimSpace(p)
}

// getPassword - Prompt for password.
func readFromSTTY(prompt string) []byte {
	fmt.Print(prompt)

	// Catch a ^C interrupt.
	// Make sure that we reset term echo before exiting.
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt)
	go func() {
		for _ = range signalChannel {
			fmt.Println("\n^C interrupt.")

			termEcho(true)
			os.Exit(1)
		}
	}()

	// Echo is disabled, now grab the data.
	termEcho(false) // disable terminal echo

	reader := bufio.NewReader(os.Stdin)
	text, err := reader.ReadString('\n')

	termEcho(true) // always re-enable terminal echo

	fmt.Println("")
	if err != nil {
		// The terminal has been reset, go ahead and exit.
		fmt.Println("ERROR:", err.Error())
		os.Exit(1)
	}

	return bytes.TrimSpace([]byte(text))
}

// techEcho() - turns terminal echo on or off.
// NOTE: the following code doesn't work in Windows system.
// how is it possible to turn echo on or off in Windows system?
func termEcho(on bool) {
	_ = on
	//// Common settings and variables for both stty calls.
	//attrs := syscall.ProcAttr{
	//	Dir:   "",
	//	Env:   []string{},
	//	Files: []uintptr{os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd()},
	//	Sys:   nil}
	//var ws syscall.WaitStatus
	//cmd := "echo"
	//if on == false {
	//	cmd = "-echo"
	//}
	//
	//// Enable/disable echoing.
	//pid, err := syscall.ForkExec(
	//	"/bin/stty",
	//	[]string{"stty", cmd},
	//	&attrs)
	//if err != nil {
	//	panic(err)
	//}
	//
	//// Wait for the stty process to complete.
	//_, err = syscall.Wait4(pid, &ws, 0, nil)
	//if err != nil {
	//	panic(err)
	//}
}
