package main

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
)

const (
	requestUrlNew    = "http://api.telegram.org/bot%s/sendMessage?chat_id=%s&text=%s&parse_mode=HTML"
	requestUrlUpdate = "http://api.telegram.org/bot%s/editMessageText?chat_id=%s&text=%s&message_id=%d&parse_mode=HTML"
)

type APIResponse struct {
	Ok          bool     `json:"ok"`
	Result      *Message `json:"result"`
	ErrorCode   int      `json:"error_code"`
	Description string   `json:"description"`
}

type Message struct {
	MessageID int `json:"message_id"`
}

var (
	botEnabled = false
	messageId  = 0
	token      string
	channelId  string
)

func BotInit(t string, c string) {
	token = t
	channelId = c

	file, err := os.Open(messageIdFile)
	if err != nil {
		return
	}
	defer file.Close()

	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&messageId)
	if err != nil {
		messageId = 0
	}

}

func saveMessageId() {
	file, err := os.OpenFile(messageIdFile, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return
	}
	defer file.Close()
	encoder := gob.NewEncoder(file)
	_ = encoder.Encode(messageId)
}

func BotSendMessage(text string) error {
	var reqUrl string

	if messageId > 0 {
		reqUrl = fmt.Sprintf(requestUrlUpdate, url.QueryEscape(token), channelId, url.QueryEscape(text), messageId)
	} else {
		reqUrl = fmt.Sprintf(requestUrlNew, url.QueryEscape(token), channelId, url.QueryEscape(text))
	}
	log.Printf("Calling with URL: %s\n", reqUrl)
	resp, err := http.Get(reqUrl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var msgResponse APIResponse
	err = json.Unmarshal(respBytes, &msgResponse)
	if err != nil {
		return err
	}

	if !msgResponse.Ok {
		return fmt.Errorf("TelegramAPI-Error %d: %s", msgResponse.ErrorCode, msgResponse.Description)
	}

	if msgResponse.Ok && messageId != msgResponse.Result.MessageID {
		messageId = msgResponse.Result.MessageID
		saveMessageId()
	}
	return nil
}
