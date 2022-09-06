package log

import (
	"fmt"
	"strings"

	"github.com/brynbellomy/klog"
	"github.com/fatih/color"
)

// Logger abstracts basic logging functions.
type Logger interface {
	SetLogLabel(inLabel string)
	GetLogLabel() string
	GetLogPrefix() string
	Debug(args ...any)
	Debugf(inFormat string, args ...any)
	Debugw(inFormat string, fields ...any)
	Success(args ...any)
	Successf(inFormat string, args ...any)
	Successw(inFormat string, fields ...any)
	LogV(inVerboseLevel int32) bool
	Info(args ...any)
	Infof(inFormat string, args ...any)
	Infow(inFormat string, fields ...any)
	Warn(args ...any)
	Warnf(inFormat string, args ...any)
	Warnw(inFormat string, fields ...any)
	Error(args ...any)
	Errorf(inFormat string, args ...any)
	Errorw(inFormat string, fields ...any)
	Fatalf(inFormat string, args ...any)
}

type logger struct {
	hasPrefix bool
	logPrefix string
	logLabel  string
}

var longestLabel int

// NewLogger creates and inits a new Logger with the given label.
func NewLogger(label string) Logger {
	l := &logger{}
	if label != "" {
		l.SetLogLabel(label)
	}
	return l
}

// Fatalf -- see Fatalf (above)
func Fatalf(inFormat string, args ...any) {
	gLogger.Fatalf(inFormat, args...)
}

var gLogger = logger{}

// SetLogLabel sets the label prefix for all entries logged.
func (l *logger) SetLogLabel(inLabel string) {
	l.logLabel = inLabel
	l.hasPrefix = len(inLabel) > 0
	if l.hasPrefix {
		l.logPrefix = fmt.Sprintf("[%s] ", inLabel)
		if len(l.logPrefix) > longestLabel {
			longestLabel = len(l.logPrefix)
		}
	}
}

// GetLogLabel returns the label last set via SetLogLabel()
func (l *logger) GetLogLabel() string {
	return l.logLabel
}

// GetLogPrefix returns the the text that prefixes all log messages for this context.
func (l *logger) GetLogPrefix() string {
	return l.logPrefix
}

// LogV returns true if logging is currently enabled for log verbose level.
func (l *logger) LogV(inVerboseLevel int32) bool {
	return bool(klog.V(klog.Level(inVerboseLevel)))
}

func (l *logger) Debug(args ...any) {
	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.DebugDepth(1, l.logPrefix, padding, fmt.Sprint(args...))
	} else {
		klog.DebugDepth(1, args...)
	}
}

func (l *logger) Debugf(inFormat string, args ...any) {
	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.DebugDepth(1, l.logPrefix, padding, fmt.Sprintf(inFormat, args...))
	} else {
		klog.DebugDepth(1, fmt.Sprintf(inFormat, args...))
	}
}

func (l *logger) Debugw(msg string, fields ...any) {
	if len(fields)%2 == 1 {
		fields = append(fields, "<MISSING>")
	}
	var fieldsStr string
	for i := 0; i < len(fields); i += 2 {
		d := color.New(color.Faint)
		fieldsStr += fmt.Sprintf("%v%v", d.Sprintf("%v=", fields[i]), fields[i+1]) + " "
	}

	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.DebugDepth(1, l.logPrefix, padding, fmt.Sprintf(msg+" %v", fieldsStr[:len(fieldsStr)-1]))
	} else {
		klog.DebugDepth(1, fmt.Sprintf(msg+" %v", fieldsStr[:len(fieldsStr)-1]))
	}
}

func (l *logger) Success(args ...any) {
	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.SuccessDepth(1, l.logPrefix, padding, fmt.Sprint(args...))
	} else {
		klog.SuccessDepth(1, args...)
	}
}

func (l *logger) Successf(inFormat string, args ...any) {
	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.SuccessDepth(1, l.logPrefix, padding, fmt.Sprintf(inFormat, args...))
	} else {
		klog.SuccessDepth(1, fmt.Sprintf(inFormat, args...))
	}
}

func (l *logger) Successw(msg string, fields ...any) {
	if len(fields)%2 == 1 {
		fields = append(fields, "<MISSING>")
	}
	var fieldsStr string
	for i := 0; i < len(fields); i += 2 {
		d := color.New(color.Faint)
		fieldsStr += fmt.Sprintf("%v%v", d.Sprintf("%v=", fields[i]), fields[i+1]) + " "
	}

	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.SuccessDepth(1, l.logPrefix, padding, fmt.Sprintf(msg+" %v", fieldsStr[:len(fieldsStr)-1]))
	} else {
		klog.SuccessDepth(1, fmt.Sprintf(msg+" %v", fieldsStr[:len(fieldsStr)-1]))
	}
}

// Info logs to the INFO log.
// Arguments are handled like fmt.Print(); a newline is appended if missing.
//
// Verbose level conventions:
//   0. Enabled during production and field deployment.  Use this for important high-level info.
//   1. Enabled during testing and development. Use for high-level changes in state, mode, or connection.
//   2. Enabled during low-level debugging and troubleshooting.
func (l *logger) Info(args ...any) {
	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.InfoDepth(1, l.logPrefix, padding, fmt.Sprint(args...))
	} else {
		klog.InfoDepth(1, args...)
	}
}

// Infof logs to the INFO log.
// Arguments are handled like fmt.Printf(); a newline is appended if missing.
//
// See comments above for Info() for guidelines for inVerboseLevel.
func (l *logger) Infof(inFormat string, args ...any) {
	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.InfoDepth(1, l.logPrefix, padding, fmt.Sprintf(inFormat, args...))
	} else {
		klog.InfoDepth(1, fmt.Sprintf(inFormat, args...))
	}
}

func (l *logger) Infow(msg string, fields ...any) {
	if len(fields)%2 == 1 {
		fields = append(fields, "<MISSING>")
	}
	var fieldsStr string
	for i := 0; i < len(fields); i += 2 {
		d := color.New(color.Faint)
		fieldsStr += fmt.Sprintf("%v%v", d.Sprintf("%v=", fields[i]), fields[i+1]) + " "
	}

	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.InfoDepth(1, l.logPrefix, padding, fmt.Sprintf(msg+" %v", fieldsStr[:len(fieldsStr)-1]))
	} else {
		klog.InfoDepth(1, fmt.Sprintf(msg+" %v", fieldsStr[:len(fieldsStr)-1]))
	}
}

// Warn logs to the WARNING and INFO logs.
// Arguments are handled like fmt.Print(); a newline is appended if missing.
//
// Warnings are reserved for situations that indicate an inconsistency or an error that
// won't result in a departure of specifications, correctness, or expected behavior.
func (l *logger) Warn(args ...any) {
	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.WarningDepth(1, l.logPrefix, padding, fmt.Sprint(args...))
	} else {
		klog.WarningDepth(1, args...)
	}
}

// Warnf logs to the WARNING and INFO logs.
// Arguments are handled like fmt.Printf(); a newline is appended if missing.
//
// See comments above for Warn() for guidelines on errors vs warnings.
func (l *logger) Warnf(inFormat string, args ...any) {
	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.WarningDepth(1, l.logPrefix, padding, fmt.Sprintf(inFormat, args...))
	} else {
		klog.WarningDepth(1, fmt.Sprintf(inFormat, args...))
	}
}

func (l *logger) Warnw(msg string, fields ...any) {
	if len(fields)%2 == 1 {
		fields = append(fields, "<MISSING>")
	}
	var fieldsStr string
	for i := 0; i < len(fields); i += 2 {
		d := color.New(color.Faint)
		fieldsStr += fmt.Sprintf("%v%v", d.Sprintf("%v=", fields[i]), fields[i+1]) + " "
	}

	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.WarningDepth(1, l.logPrefix, padding, fmt.Sprintf(msg+" %v", fieldsStr[:len(fieldsStr)-1]))
	} else {
		klog.WarningDepth(1, fmt.Sprintf(msg+" %v", fieldsStr[:len(fieldsStr)-1]))
	}
}

// Error logs to the ERROR, WARNING, and INFO logs.
// Arguments are handled like fmt.Print(); a newline is appended if missing.
//
// Errors are reserved for situations that indicate an implementation deficiency, a
// corruption of data or resources, or an issue that if not addressed could spiral into deeper issues.
// Logging an error reflects that correctness or expected behavior is either broken or under threat.
func (l *logger) Error(args ...any) {
	{
		if l.hasPrefix {
			padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
			klog.ErrorDepth(1, l.logPrefix, padding, fmt.Sprint(args...))
		} else {
			klog.ErrorDepth(1, args...)
		}
	}
}

// Errorf logs to the ERROR, WARNING, and INFO logs.
// Arguments are handled like fmt.Print; a newline is appended if missing.
//
// See comments above for Error() for guidelines on errors vs warnings.
func (l *logger) Errorf(inFormat string, args ...any) {
	{
		if l.hasPrefix {
			padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
			klog.ErrorDepth(1, l.logPrefix, padding, fmt.Sprintf(inFormat, args...))
		} else {
			klog.ErrorDepth(1, fmt.Sprintf(inFormat, args...))
		}
	}
}

func (l *logger) Errorw(msg string, fields ...any) {
	if len(fields)%2 == 1 {
		fields = append(fields, "<MISSING>")
	}
	var fieldsStr string
	for i := 0; i < len(fields); i += 2 {
		d := color.New(color.Faint)
		fieldsStr += fmt.Sprintf("%v%v", d.Sprintf("%v=", fields[i]), fields[i+1]) + " "
	}

	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.ErrorDepth(1, l.logPrefix, padding, fmt.Sprintf(msg+" %v", fieldsStr[:len(fieldsStr)-1]))
	} else {
		klog.ErrorDepth(1, fmt.Sprintf(msg+" %v", fieldsStr[:len(fieldsStr)-1]))
	}
}

// Fatalf logs to the FATAL, ERROR, WARNING, and INFO logs,
// Arguments are handled like fmt.Printf(); a newline is appended if missing.
func (l *logger) Fatalf(inFormat string, args ...any) {
	{
		if l.hasPrefix {
			padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
			klog.FatalDepth(1, l.logPrefix, padding, fmt.Sprintf(inFormat, args...))
		} else {
			klog.FatalDepth(1, fmt.Sprintf(inFormat, args...))
		}
	}
}
