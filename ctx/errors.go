package ctx

type ctxErr struct{ msg string }

func (err *ctxErr) Error() string {
	return err.msg
}

// Errors
var (
    // ctx.Context
    ErrCtxNotRunning = &ctxErr{"context not running"}
)

