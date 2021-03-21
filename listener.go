package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/aws/aws-sdk-go/service/ssm"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type BotIdentifier struct {
	name  string
	token string
}

type BotUpdate struct {
	name   string
	update tgbotapi.Update
}

var snsclient *sns.SNS
var ssmclient *ssm.SSM

const tokenDirectory = "/telegram/token/"
const topicArn = "/telegram/topic/command"

func init() {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	snsclient = sns.New(sess)
	ssmclient = ssm.New(sess)
}

func main() {

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	log.Printf("Listening on port %s", port)
	go http.ListenAndServe(fmt.Sprintf(":%s", port), nil)

	botIds := getBotIdentifiers()
	botUpdates := make(chan BotUpdate)

	for _, botId := range botIds {
		go func(botId BotIdentifier) {
			for update := range getUpdatesChannel(botId.token) {
				botUpdates <- BotUpdate{botId.name, update}
			}
		}(*botId)
	}

	for botupdate := range botUpdates {
		updateBytes, _ := json.Marshal(&botupdate.update)
		updateString := string(updateBytes)
		if botupdate.update.Message.Command() == "" {
			continue
		}
		topicArn := getCommandTopicArn()
		output, err := snsclient.Publish(&sns.PublishInput{
			Message:  &updateString,
			TopicArn: &topicArn,
			MessageAttributes: map[string]*sns.MessageAttributeValue{
				"bot": {
					DataType:    aws.String("String"),
					StringValue: aws.String(botupdate.name),
				},
				"command": {
					DataType:    aws.String("String"),
					StringValue: aws.String(botupdate.update.Message.Command()),
				},
			},
		})
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Published to sns: %s", *output.MessageId)
		log.Printf("BotUpdate: %+v", botupdate)
	}

}

func getCommandTopicArn() string {
	result, _ := ssmclient.GetParameter(&ssm.GetParameterInput{
		Name: aws.String(topicArn),
	})
	return *result.Parameter.Value
}

func getBotIdentifiers() []*BotIdentifier {
	parameters, _ := ssmclient.GetParametersByPath(&ssm.GetParametersByPathInput{
		Path:           aws.String(tokenDirectory),
		WithDecryption: aws.Bool(true),
	})
	bots := []*BotIdentifier{}
	for _, parameter := range parameters.Parameters {
		bots = append(bots, &BotIdentifier{
			name:  strings.TrimPrefix(*parameter.Name, tokenDirectory),
			token: *parameter.Value,
		})
	}
	return bots
}

func getUpdatesChannel(token string) tgbotapi.UpdatesChannel {
	botApi, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}

	botApi.Debug = false

	botUrl, _ := url.Parse(os.Getenv("APP_URL"))
	botUrl.Path = token

	_, err = botApi.Request(&tgbotapi.WebhookConfig{
		URL: botUrl,
	})

	if err != nil {
		log.Fatal(err)
	}
	info, err := botApi.GetWebhookInfo()
	if err != nil {
		log.Fatal(err)
	}
	if info.LastErrorDate != 0 {
		log.Printf("Telegram callback failed: %s", info.LastErrorMessage)
	}
	return botApi.ListenForWebhook("/" + botApi.Token)
}
