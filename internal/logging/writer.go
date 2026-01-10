package logging

var storageInstance *LogStorage

// InitLogHandler initializes the log storage system
// With the wrapper approach, logs are captured directly via CaptureLog in wrapper functions
func InitLogHandler() {
	storageInstance = GetStorage()
}

// CaptureLog manually captures a log entry (for use in wrapper functions)
func CaptureLog(level, msg string, args ...interface{}) {
	if storageInstance == nil {
		// Ensure storage is initialized
		storageInstance = GetStorage()
	}

	fields := make(map[string]interface{})

	// Parse key-value pairs from args
	for i := 0; i < len(args)-1; i += 2 {
		if key, ok := args[i].(string); ok {
			value := args[i+1]
			// Convert error types to their string representation
			if err, ok := value.(error); ok {
				fields[key] = err.Error()
			} else {
				fields[key] = value
			}
		}
	}

	// If there's an odd number of args, the last one might be an error
	if len(args)%2 == 1 {
		if err, ok := args[len(args)-1].(error); ok {
			fields["error"] = err.Error()
		}
	}

	storageInstance.AddEntry(level, msg, fields)
}
