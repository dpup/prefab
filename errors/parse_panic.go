package errors

import (
	"strconv"
	"strings"
)

type uncaughtPanicError struct {
	message string
}

func (p uncaughtPanicError) Error() string {
	return p.message
}

type state string

const (
	startState   state = "start"
	seekState    state = "seek"
	parsingState state = "parsing"
	doneState    state = "done"
)

// ParseStack is intended to be used with the results of `debug.Stack` this is
// a hack for now and should be improved.
func ParseStack(b []byte) (*Error, error) {
	return ParsePanic("panic: hack\n\n" + string(b))
}

// ParsePanic allows you to get an error object from the output of a go program
// that panicked. This is particularly useful with https://github.com/mitchellh/panicwrap.
//
//nolint:gocognit // This function is complex by nature.
func ParsePanic(text string) (*Error, error) {
	lines := strings.Split(text, "\n")

	state := startState

	var message string
	var stack []StackFrame

	for i := 0; i < len(lines) && state != doneState; i++ {
		line := lines[i]

		switch state {
		case startState:
			if strings.HasPrefix(line, "panic: ") {
				message = strings.TrimPrefix(line, "panic: ")
				state = seekState
			} else {
				return nil, Errorf("bugsnag.panicParser: Invalid line (no prefix): %s", line)
			}
		case seekState:
			if strings.HasPrefix(line, "goroutine ") && strings.HasSuffix(line, "[running]:") {
				state = parsingState
			}
		case parsingState:
			if line == "" {
				state = doneState
				continue
			}
			createdBy := false
			if strings.HasPrefix(line, "created by ") {
				line = strings.TrimPrefix(line, "created by ")
				createdBy = true
			}

			i++

			if i >= len(lines) {
				return nil, Errorf("bugsnag.panicParser: Invalid line (unpaired): %s", line)
			}

			frame, err := parsePanicFrame(line, lines[i], createdBy)
			if err != nil {
				return nil, err
			}

			stack = append(stack, *frame)
			if createdBy {
				state = doneState
			}
		}
	}

	if state == doneState || state == parsingState {
		return &Error{Err: uncaughtPanicError{message}, frames: stack}, nil
	}
	return nil, Errorf("could not parse panic: %v", text)
}

// The lines we're passing look like this:
//
//	main.(*foo).destruct(0xc208067e98)
//	        /0/go/src/github.com/bugsnag/bugsnag-go/pan/main.go:22 +0x151
func parsePanicFrame(name string, line string, createdBy bool) (*StackFrame, error) {
	idx := strings.LastIndex(name, "(")
	if idx == -1 && !createdBy {
		return nil, Errorf("bugsnag.panicParser: Invalid line (no call): %s", name)
	}
	if idx != -1 {
		name = name[:idx]
	}
	pkg := ""

	if lastslash := strings.LastIndex(name, "/"); lastslash >= 0 {
		pkg += name[:lastslash] + "/"
		name = name[lastslash+1:]
	}
	if period := strings.Index(name, "."); period >= 0 {
		pkg += name[:period]
		name = name[period+1:]
	}

	name = strings.ReplaceAll(name, "·", ".")

	if !strings.HasPrefix(line, "\t") {
		return nil, Errorf("bugsnag.panicParser: Invalid line (no tab): %s", line)
	}

	idx = strings.LastIndex(line, ":")
	if idx == -1 {
		return nil, Errorf("bugsnag.panicParser: Invalid line (no line number): %s", line)
	}
	file := line[1:idx]

	number := line[idx+1:]
	if idx = strings.Index(number, " +"); idx > -1 {
		number = number[:idx]
	}

	lno, err := strconv.ParseInt(number, 10, 32)
	if err != nil {
		return nil, Errorf("bugsnag.panicParser: Invalid line (bad line number): %s", line)
	}

	return &StackFrame{
		File:       file,
		LineNumber: int(lno),
		Package:    pkg,
		Name:       name,
	}, nil
}
