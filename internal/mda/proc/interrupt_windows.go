//go:build windows

package proc

import "os"

// terminationSignals is empty: Windows ends console processes without a
// signal to intercept.
var terminationSignals []os.Signal

func terminate(os.Signal) {}
