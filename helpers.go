package helpers

func Assert(condition bool) {
	if !condition {
		panic("Assertion failed")
	}
}
