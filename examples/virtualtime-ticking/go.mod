module github.com/d-led/phony/examples/virtualtime-ticking

go 1.19

require github.com/d-led/phony v0.0.5

require go.uber.org/goleak v1.2.1

// This example builds against the working copy of the library, so that a change to
// the library is tested here first. Delete this line to build against the
// published module instead.
replace github.com/d-led/phony => ../..
