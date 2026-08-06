package simplepkg

import "example.internal/my/other"

const _bar = "42"

func bar() {
	other.Bar()
}
