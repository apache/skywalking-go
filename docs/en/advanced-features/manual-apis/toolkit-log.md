# Logging APIs

## Add Logging Toolkit

toolkit/logging provides the APIs to attaching log information to the Span in the current context, such as debug, info, warn, error.
Add the toolkit/logging dependency to your project.

```go
import "github.com/apache/skywalking-go/toolkit/logging"
```

## Use Native Logging

toolkit/logging provides common log level APIs. We need to pass a required "Message" parameter and multiple optional string type key-value pairs.

```go
// Debug logs a message at DebugLevel
func Debug(msg string, keyValues ...string) 

// Info logs a message at InfoLevel
func Info(msg string, keyValues ...string) 

// Warn logs a message at DebugLevel
func Warn(msg string, keyValues ...string) 

// Error logs a message at ErrorLevel
func Error(msg string, keyValues ...string)
```

### Associate Span

When we call logging APIs to log, it will attach the log information to the active Span in the current Context. Even if we log across different Goroutines, it can correctly attach the log to span.

### Example

We create a LocalSpan in the `main` func, and we call `logging.debug` to record a log. The log will be attached to the Span of the current context.

```go
func main() {
    span, err := trace.CreateLocalSpan("foo")
    if err != nil {
        log.Fatalln(err)
    }
	
    logging.Debug("this is debug info", "foo", "bar")
}
```