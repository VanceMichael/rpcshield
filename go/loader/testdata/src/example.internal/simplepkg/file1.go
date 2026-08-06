package simplepkg

import "example.internal/my/pkg"

const _foo = 42

func foo() {
	pkg.Bar()
}
