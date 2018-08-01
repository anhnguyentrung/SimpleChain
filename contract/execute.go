package main

import (
	"os/exec"
	"fmt"
	"os"
)

func main() {
	args := os.Args[1:]
	cmdArgs := []string{"wasm_exec.js"}
	cmdArgs = append(cmdArgs, args...)
	out, err := exec.Command("node", cmdArgs...).Output()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("結果: %s", out)
}
