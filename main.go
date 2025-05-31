package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Azure/go-amqp"
	"github.com/gin-gonic/gin"
)

const (
	brokerUrlVariable        = "ASB_BROKER_URL"
	accessKeyNameVariable    = "ASB_ACCESS_KEY_NAME"
	accessKeyVariable        = "ASB_ACCESS_KEY"
	topicVariable            = "ASB_TOPIC"
	subscriptionNameVariable = "ASB_SUBSCRIPTION"
)

func main() {
	logger := log.New(os.Stdout, "[AMQP] ", log.LstdFlags|log.Lshortfile)
	logger.Println("Starting AMQP Publisher-Subscriber application")

	config, err := loadConfigs()
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	publisher, cleanupPub, err := NewPublisher(ctx, logger, config)
	if err != nil {
		logger.Fatalf("Publisher init failed: %v", err)
	}
	defer cleanupPub()

	// Init Subscriber
	subscriber, cleanupSub, err := NewSubscriber(ctx, logger, config)
	if err != nil {
		logger.Fatalf("Subscriber init failed: %v", err)
	}
	defer cleanupSub()

	go func() {
		if err := subscriber.StartListening(ctx); err != nil {
			logger.Fatalf("Subscriber error: %v", err)
		}
	}()
	logger.Println("Subscriber started successfully")

	router := gin.New()
	router.POST("/publish", publisher.handlePublish)

	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go func() {
		logger.Println("HTTP server started on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Server failed: %v", err)
		}
	}()

	<-sigChan
	logger.Println("Shutdown signal received")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Fatalf("HTTP server Shutdown failed: %v", err)
	}
	logger.Println("Gracefully shut down")
}

type AmqpConfig struct {
	ConnectionString string
	Topic            string
	Subscription     string
}

func loadConfigs() (AmqpConfig, error) {
	brokerUrl := os.Getenv(brokerUrlVariable)
	accessKeyName := os.Getenv(accessKeyNameVariable)
	accessKey := os.Getenv(accessKeyVariable)
	topic := os.Getenv(topicVariable)
	subscriptionName := os.Getenv(subscriptionNameVariable)

	if brokerUrl == "" || accessKeyName == "" || accessKey == "" || topic == "" || subscriptionName == "" {
		return AmqpConfig{}, fmt.Errorf("missing one or more required environment variables")
	}

	encodedKey := url.QueryEscape(accessKey)
	connectionString := fmt.Sprintf("amqps://%s:%s@%s", accessKeyName, encodedKey, brokerUrl)

	subscription := fmt.Sprintf("%s/subscriptions/%s", topic, subscriptionName)

	return AmqpConfig{
		ConnectionString: connectionString,
		Topic:            topic,
		Subscription:     subscription,
	}, nil
}

type Publisher struct {
	sender *amqp.Sender
	logger *log.Logger
}

func NewPublisher(ctx context.Context, logger *log.Logger, config AmqpConfig) (*Publisher, func(), error) {
	conn, err := amqp.Dial(ctx, config.ConnectionString, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to AMQP broker: %w", err)
	}

	session, err := conn.NewSession(ctx, nil)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("failed to create AMQP session: %w", err)
	}

	sender, err := session.NewSender(ctx, config.Topic, nil)
	if err != nil {
		session.Close(ctx)
		conn.Close()
		return nil, nil, fmt.Errorf("failed to create AMQP sender: %w", err)
	}

	cleanup := func() {
		sender.Close(ctx)
		session.Close(ctx)
		conn.Close()
	}

	return &Publisher{sender: sender, logger: logger}, cleanup, nil
}

func (p *Publisher) Publish(ctx context.Context, message string) error {
	msg := amqp.NewMessage([]byte(message))
	if err := p.sender.Send(ctx, msg, nil); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	p.logger.Printf("Published message: %s", message)
	return nil
}

func (p *Publisher) handlePublish(c *gin.Context) {
	var req PublishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}
	if err := p.Publish(c, req.Message); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to publish message"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "Message published"})
}

type Subscriber struct {
	receiver *amqp.Receiver
	logger   *log.Logger
}

func NewSubscriber(ctx context.Context, logger *log.Logger, config AmqpConfig) (*Subscriber, func(), error) {
	conn, err := amqp.Dial(ctx, config.ConnectionString, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to AMQP broker: %w", err)
	}

	session, err := conn.NewSession(ctx, nil)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("failed to create AMQP session: %w", err)
	}

	receiver, err := session.NewReceiver(ctx, config.Subscription, nil)
	if err != nil {
		session.Close(ctx)
		conn.Close()
		return nil, nil, fmt.Errorf("failed to create AMQP receiver: %w", err)
	}

	cleanup := func() {
		receiver.Close(ctx)
		session.Close(ctx)
		conn.Close()
	}

	return &Subscriber{receiver: receiver, logger: logger}, cleanup, nil
}

func (s *Subscriber) StartListening(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			s.logger.Println("Subscriber shutting down...")
			return nil
		default:
			msg, err := s.receiver.Receive(ctx, nil)
			if err != nil {
				return fmt.Errorf("failed to receive message: %w", err)
			}
			s.logger.Printf("Received message: %s", string(msg.GetData()))
			if err := s.receiver.AcceptMessage(ctx, msg); err != nil {
				return fmt.Errorf("failed to accept message: %w", err)
			}
		}
	}
}

type PublishRequest struct {
	Message string `json:"message"`
}
