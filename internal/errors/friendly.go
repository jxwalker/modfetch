package errors

import (
	"fmt"
	"strings"
)

// UserFriendlyError provides actionable error messages for end users
type UserFriendlyError struct {
	Message    string // User-facing message explaining what went wrong
	Suggestion string // Actionable steps to fix the issue
	DocsLink   string // Optional link to documentation
	Details    error  // Original error for debugging/logs
}

func (e *UserFriendlyError) Error() string {
	var sb strings.Builder
	sb.WriteString(e.Message)

	if e.Suggestion != "" {
		sb.WriteString("\n\n")
		sb.WriteString("How to fix:\n")
		sb.WriteString(e.Suggestion)
	}

	if e.DocsLink != "" {
		sb.WriteString("\n\n")
		sb.WriteString("Documentation: ")
		sb.WriteString(e.DocsLink)
	}

	return sb.String()
}

func (e *UserFriendlyError) Unwrap() error {
	return e.Details
}

// NewFriendlyError creates a user-friendly error
func NewFriendlyError(message, suggestion string) *UserFriendlyError {
	return &UserFriendlyError{
		Message:    message,
		Suggestion: suggestion,
	}
}

// WithDetails adds the underlying error details
func (e *UserFriendlyError) WithDetails(err error) *UserFriendlyError {
	e.Details = err
	return e
}

// WithDocs adds a documentation link
func (e *UserFriendlyError) WithDocs(link string) *UserFriendlyError {
	e.DocsLink = link
	return e
}

// Common error constructors for frequently encountered issues

// NetworkError returns a network-related error with helpful suggestions
func NetworkError(err error) *UserFriendlyError {
	msg := "Network error occurred"
	suggestion := "Check your internet connection and try again"

	if err != nil {
		errStr := err.Error()

		// DNS resolution failure
		if strings.Contains(errStr, "no such host") || strings.Contains(errStr, "name resolution") {
			msg = "Cannot resolve hostname - DNS lookup failed"
			suggestion = "1. Check your internet connection\n2. Verify DNS settings\n3. Try: ping google.com"
		}

		// Connection refused
		if strings.Contains(errStr, "connection refused") {
			msg = "Server refused connection"
			suggestion = "The server may be down or blocking requests. Try again later."
		}

		// Timeout
		if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
			msg = "Connection timed out"
			suggestion = "Server is slow or unreachable. Try:\n1. Increase timeout with --timeout flag\n2. Check your network speed\n3. Try again later"
		}

		// Certificate errors
		if strings.Contains(errStr, "certificate") || strings.Contains(errStr, "x509") {
			msg = "SSL/TLS certificate verification failed"
			suggestion = "You may be behind a corporate proxy. Try:\n  export REQUESTS_CA_BUNDLE=/path/to/cert.pem\nOr disable verification (insecure): add tls_verify: false to config"
		}
	}

	return &UserFriendlyError{
		Message:    msg,
		Suggestion: suggestion,
		Details:    err,
	}
}

// AuthError returns authentication-related errors with token setup guidance
func AuthError(host string, statusCode int, err error) *UserFriendlyError {
	msg := fmt.Sprintf("Authentication failed (%d)", statusCode)
	suggestion := "Check your access token"

	switch {
	case strings.Contains(host, "huggingface.co"):
		msg = "HuggingFace authentication failed"
		suggestion = "1. Set your token: export HF_TOKEN=hf_...\n" +
			"2. Get a token at: https://huggingface.co/settings/tokens\n" +
			"3. Ensure you have accepted the repository license"

	case strings.Contains(host, "civitai.com"):
		msg = "CivitAI authentication failed"
		suggestion = "1. Set your token: export CIVITAI_TOKEN=...\n" +
			"2. Get a token at: https://civitai.com/user/account\n" +
			"3. Ensure the model is publicly accessible"
	}

	return &UserFriendlyError{
		Message:    msg,
		Suggestion: suggestion,
		Details:    err,
	}
}

// DiskSpaceError returns disk space related errors
func DiskSpaceError(availableBytes, requiredBytes uint64) *UserFriendlyError {
	return &UserFriendlyError{
		Message: fmt.Sprintf("Insufficient disk space: need %s but only %s available",
			formatBytes(requiredBytes),
			formatBytes(availableBytes)),
		Suggestion: fmt.Sprintf("Free up at least %s of disk space and try again",
			formatBytes(requiredBytes-availableBytes)),
	}
}

// ConfigError returns configuration-related errors
func ConfigError(field, issue string) *UserFriendlyError {
	return &UserFriendlyError{
		Message:    fmt.Sprintf("Configuration error in field '%s': %s", field, issue),
		Suggestion: "Run 'modfetch config validate' to check your configuration\nOr run 'modfetch setup' to create a new configuration interactively",
		DocsLink:   "https://github.com/jxwalker/modfetch#configuration",
	}
}

// DatabaseError returns database-related errors with recovery suggestions
func DatabaseError(err error) *UserFriendlyError {
	msg := "Database error"
	suggestion := "Try running: modfetch doctor"

	if err != nil {
		errStr := err.Error()

		if strings.Contains(errStr, "locked") {
			msg = "Database is locked by another process"
			suggestion = "Close other modfetch instances and try again\nOr remove lock file: ~/.config/modfetch/modfetch.lock"
		}

		if strings.Contains(errStr, "corrupt") || strings.Contains(errStr, "malformed") {
			msg = "Database is corrupted"
			suggestion = "Backup and repair the database:\n" +
				"1. cp ~/.config/modfetch/state.db ~/.config/modfetch/state.db.backup\n" +
				"2. modfetch doctor --repair"
		}
	}

	return &UserFriendlyError{
		Message:    msg,
		Suggestion: suggestion,
		Details:    err,
	}
}

// PathError returns file/directory path related errors
func PathError(path string, err error) *UserFriendlyError {
	msg := fmt.Sprintf("Path error: %s", path)
	suggestion := "Check that the path exists and you have permission to access it"

	if err != nil {
		errStr := err.Error()

		if strings.Contains(errStr, "permission denied") {
			msg = fmt.Sprintf("Permission denied: %s", path)
			suggestion = fmt.Sprintf("Ensure you have write permission:\n  chmod u+w %s", path)
		}

		if strings.Contains(errStr, "no such file or directory") {
			msg = fmt.Sprintf("Directory does not exist: %s", path)
			suggestion = fmt.Sprintf("Create the directory:\n  mkdir -p %s", path)
		}

		if strings.Contains(errStr, "not a directory") {
			msg = fmt.Sprintf("Path exists but is not a directory: %s", path)
			suggestion = "Remove the file or choose a different path"
		}
	}

	return &UserFriendlyError{
		Message:    msg,
		Suggestion: suggestion,
		Details:    err,
	}
}

// formatBytes converts bytes to human-readable format
func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
