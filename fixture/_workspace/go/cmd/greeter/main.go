// Command greeter prints a greeting.
package main

import (
	"fmt"
	"os"

	"example.com/hashbench/go/greet"
)

func main() {
	name := "world"
	if len(os.Args) > 1 {
		name = os.Args[1]
	}
	fmt.Println(greet.Greeting(name))
}
