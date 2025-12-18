package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// helper function to ask for confirmation
func askForConfirmation(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [Y/n]: ", prompt)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}
