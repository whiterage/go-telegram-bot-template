package logger

import (
	"fmt"
	"log"
)

// LogErrorWithContext логирует ошибку с контекстом
func LogErrorWithContext(err error, context string, fields ...string) {
	if err == nil {
		return
	}

	var msg string
	if len(fields) > 0 {
		msg = fmt.Sprintf("%s: %v | %s", context, err, fields[0])
		for i := 1; i < len(fields); i += 2 {
			if i+1 < len(fields) {
				msg += fmt.Sprintf(" | %s: %s", fields[i], fields[i+1])
			}
		}
	} else {
		msg = fmt.Sprintf("%s: %v", context, err)
	}

	log.Printf("ERROR %s", msg)
}

// LogSendError логирует ошибку отправки сообщения
func LogSendError(err error, chatID int64, messageType string) {
	LogErrorWithContext(err, "send message failed",
		"chat_id", fmt.Sprintf("%d", chatID),
		"type", messageType)
}

// LogRequestError логирует ошибку запроса к API
func LogRequestError(err error, method string, params ...string) {
	LogErrorWithContext(err, "API request failed",
		"method", method,
		"params", fmt.Sprintf("%v", params))
}

// LogCallbackError логирует ошибку обработки callback
func LogCallbackError(err error, callbackID string, userID int64) {
	LogErrorWithContext(err, "callback processing failed",
		"callback_id", callbackID,
		"user_id", fmt.Sprintf("%d", userID))
}

// LogOrderError логирует ошибку работы с заявкой
func LogOrderError(err error, orderID int64, operation string) {
	LogErrorWithContext(err, "order operation failed",
		"order_id", fmt.Sprintf("%d", orderID),
		"operation", operation)
}

// LogBoardError логирует ошибку работы с доской
func LogBoardError(err error, orderID int64, operation string, threadID int) {
	LogErrorWithContext(err, "board operation failed",
		"order_id", fmt.Sprintf("%d", orderID),
		"operation", operation,
		"thread_id", fmt.Sprintf("%d", threadID))
}
