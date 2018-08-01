package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	filename := os.Args[1]
	fmt.Println(filename)
	src := filename + ".go"
	dst := filename + ".wasm"
	os.Setenv("GOOS", "js")
	os.Setenv("GOARCH", "wasm")
	cmdArgs := []string{"build", "-o", dst, src}
	out, err := exec.Command("go1.11beta1", cmdArgs...).Output()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("結果: %s \n", out)
}
