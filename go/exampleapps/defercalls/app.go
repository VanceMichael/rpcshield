package main

import (
	_ "context"
	_ "sync"
)

func successHasDifer() {
	defer func() {}()
}

func failureNoDiffer() {
	func() {}()
}

func main() {
}
