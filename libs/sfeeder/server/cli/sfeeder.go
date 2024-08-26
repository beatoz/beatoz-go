package main

import (
	"fmt"
	"github.com/beatoz/beatoz-go/libs/sfeeder/server"
	"os"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("sfeeder {listening_address} {store_director}")
		fmt.Println("\t{listening_address}\tip:port. (default: '0.0.0.0:')")
		fmt.Println("\t{store_director}\tdirectory which contains secret files")
		return
	}

	server.Start(os.Args[1], os.Args[2])
}
