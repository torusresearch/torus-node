package main

import (
	"fmt"

	"github.com/YZhenY/DKGNode/tmabci"
)

func main() {
	err := tmabci.RunBft()
	if err != nil {
		fmt.Println("bft failed test")
	}
}
