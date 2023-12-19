package services

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"log"
)

/// Messages are stored to the session and displayed only once

func addMessage(c *gin.Context, message string, key string) {
	session := sessions.Default(c)
	var messages []string
	sessionMessages := session.Get(key)
	if sessionMessages == nil {
		messages = make([]string, 1)
		messages[0] = message
	} else {
		var ok bool
		messages, ok = sessionMessages.([]string)
		if !ok {
			log.Printf("unknown type stored to session " + key)
			session.Delete(key)
			err := session.Save()
			if err != nil {
				log.Printf("failed to save a message to the session, %v", err)
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
		log.Printf("failed to save a message to session, %v", err)
	}
}

func AddMessage(c *gin.Context, message string) {
	addMessage(c, message, "messages")
}

func AddError(c *gin.Context, message string) {
	addMessage(c, message, "errors")
}

func AddWarning(c *gin.Context, message string) {
	addMessage(c, message, "warnings")
}

func GetMessages(c *gin.Context) map[string]any {
	session := sessions.Default(c)
	messages := session.Get("messages")
	errors := session.Get("errors")
	warnings := session.Get("warnings")
	session.Delete("messages")
	session.Delete("errors")
	session.Delete("warnings")
	err := session.Save()
	if err != nil {
		log.Printf("failed to save a message to session, %v", err)
	}

	result := gin.H{
		"Errors":   errors,
		"Warnings": warnings,
		"Messages": messages,
	}
	return result
}
