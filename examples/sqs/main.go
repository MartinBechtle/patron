package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"

	"github.com/beatlabs/patron"
	"github.com/beatlabs/patron/component/async"
	patronsqs "github.com/beatlabs/patron/component/async/sqs"
	"github.com/beatlabs/patron/log"
)

type sqsConfig struct {
	endpoint string
	name     string
	region   string
}

// Override these values and make sure you have credentials and a session token in your ~/.aws/credentials file.
// Then set the appropriate value to an AWS_PROFILE env var when running this example.
// The assumed role needs to have access to the queue.
var sampleConfig sqsConfig = sqsConfig{
	endpoint: "https://sqs.eu-west-1.amazonaws.com/315266612443/sandbox-payin",
	name:     "sandbox-payin",
	region:   "eu-west-1",
}

func init() {
	err := os.Setenv("PATRON_LOG_LEVEL", "debug")
	if err != nil {
		fmt.Printf("failed to set log level env var: %v", err)
		os.Exit(1)
	}
}

func main() {
	name := "sqs"
	version := "1.0.0"

	err := patron.SetupLogging(name, version)
	if err != nil {
		fmt.Printf("failed to set up logging: %v", err)
		os.Exit(1)
	}
	ctx := context.Background()

	sqsComponent, err := sampleSqs()
	if err != nil {
		log.Fatalf("failed to create sqs component: %v", err)
	}

	err = patron.New(name, version).
		WithComponents(sqsComponent).
		Run(ctx)
	if err != nil {
		log.Fatalf("failed to create and run service: %v", err)
	}
}

func sampleSqs() (*async.Component, error) {
	sess, err := session.NewSession(&aws.Config{
		Endpoint: &sampleConfig.endpoint,
		Region:   &sampleConfig.region,
	})
	if err != nil {
		return nil, err
	}
	sqsClient := sqs.New(sess)

	concurrency := uint(10)

	factory, err := patronsqs.NewFactory(
		sqsClient,
		sampleConfig.name,
		// Optionally override the queue's default polling setting.
		// Long polling is highly recommended to avoid large costs on AWS.
		// See https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-short-and-long-polling.html
		// It's probably best to not specify any value: the default value on the queue will be used.
		patronsqs.PollWaitSeconds(20),
		// Optionally override the queue's default visibility timeout
		// See https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-visibility-timeout.html
		// Again, a sensible default should be configured on the queue, but there might be specific use case where you want to override.
		patronsqs.VisibilityTimeout(30),
		// Optionally change the number of messages fetched by each worker.
		// The default is 3
		patronsqs.MaxMessages(5),
	)
	if err != nil {
		return nil, err
	}

	// Note: the retry count is not increased on an error processing a message, but rather consuming from the queue.
	// If the max number if retries is reached, the service will terminate.
	// The max number of retires of a message is determined by the SQS queue, not the consumer.
	return async.New("sqs", factory, messageHandler).
		WithFailureStrategy(async.NackStrategy).
		WithRetries(3).
		WithRetryWait(30 * time.Second).
		WithConcurrency(concurrency).
		Create()
}

func messageHandler(message async.Message) error {
	log.Info("Received message, payload:", string(message.Payload()))
	time.Sleep(3 * time.Second) // useful to see concurrency in action
	return nil
}
