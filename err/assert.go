package err

// Assert provides a function that is similar to the assert() function in C.
// Call it to ensure invariant conditions are met.
func Assert(condition bool) {
	if !condition {
		panic("Assertion failed")
	}
}
