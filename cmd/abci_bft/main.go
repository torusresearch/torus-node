package main

import (
	"fmt"

	"github.com/YZhenY/torus/tmabci"
)

func main() {
	err := tmabci.RunBft()
	if err != nil {
		fmt.Println("bft failed test")
	}
}
