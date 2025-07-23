package services

import (
	"fmt"
	"log/slog"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

/// Messages are stored to the session and displayed only once

func addMessage(c *gin.Context, message, key string) {
	session := sessions.Default(c)
	var messages []string
	sessionMessages := session.Get(key)

	slog.Debug("Adding message to session", "key", key, "message", message)

	if sessionMessages == nil {
		messages = make([]string, 1)
		messages[0] = message
	} else {
		var ok bool
		messages, ok = sessionMessages.([]string)
		if !ok {
			slog.Error("Unknown type stored to session", "key", key, "type", typeof(sessionMessages))
			session.Delete(key)
			err := session.Save()
			if err != nil {
				slog.Error("Failed to save session after deleting invalid value", "key", key, "error", err)
			}
			return
		}
		if messages == nil {
			messages = make([]string, 1)
			messages[0] = message
		} else {
			messages = append(messages, message)
		}
	}

	session.Set(key, messages)
	err := session.Save()
	if err != nil {
		slog.Error("Failed to save message to session", "key", key, "error", err)
	}
}

// Helper function to get type name as string for logging
func typeof(v interface{}) string {
	if v == nil {
		return "nil"
	}
	return fmt.Sprintf("%T", v)
}

func AddMessage(c *gin.Context, message string) {
	slog.Info("Adding info message", "message", message)
	addMessage(c, message, "messages")
}

func AddError(c *gin.Context, message string) {
	slog.Info("Adding error message", "message", message)
	addMessage(c, message, "errors")
}

func AddWarning(c *gin.Context, message string) {
	slog.Info("Adding warning message", "message", message)
	addMessage(c, message, "warnings")
}

func GetMessages(c *gin.Context) map[string]any {
	session := sessions.Default(c)
	messages := session.Get("messages")
	errors := session.Get("errors")
	warnings := session.Get("warnings")

	// Log counts of each message type for debugging purposes
	messageCount := countMessages(messages)
	errorCount := countMessages(errors)
	warningCount := countMessages(warnings)

	slog.Debug("Retrieved messages from session",
		"messageCount", messageCount,
		"errorCount", errorCount,
		"warningCount", warningCount)

	session.Delete("messages")
	session.Delete("errors")
	session.Delete("warnings")
	err := session.Save()
	if err != nil {
		slog.Error("Failed to save session after retrieving messages", "error", err)
	}

	result := gin.H{
		"Errors":   errors,
		"Warnings": warnings,
		"Messages": messages,
	}
	return result
}

// Helper function to count messages in a session value
func countMessages(sessionValue interface{}) int {
	if sessionValue == nil {
		return 0
	}
	messages, ok := sessionValue.([]string)
	if !ok {
		return 0
	}
	return len(messages)
}
