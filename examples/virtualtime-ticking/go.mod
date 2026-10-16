module github.com/d-led/phony/examples/virtualtime-ticking

go 1.19

require github.com/d-led/phony v0.1.0

// The fork is not tagged yet, so this example builds against the working copy.
// Delete this line to build against the published module once there is a tag.
replace github.com/d-led/phony => ../..
