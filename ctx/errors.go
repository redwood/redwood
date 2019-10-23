package ctx

type ctxErr struct{ msg string }

func (err *ctxErr) Error() string {
	return err.msg
}

var ErrCtxNotRunning = &ctxErr{"context not running"}
