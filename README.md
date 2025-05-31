# AMQP Pub-Sub with Azure Service Bus in Go

This is a demo project showcasing a simple **Publisher-Subscriber** implementation using [**Azure Service Bus**](https://learn.microsoft.com/en-us/azure/service-bus-messaging/service-bus-messaging-overview) over **AMQP** with the [`go-amqp`](github.com/Azure/go-amqp) library. It includes:

- A REST API built using Gin to publish messages
- A background subscriber that listens to messages from a Service Bus topic subscription

## Features

- Publishes messages via HTTP POST (`/publish`)
- Subscribes and logs messages received from the Azure Service Bus topic subscription

---

## Prerequisites

- Go 1.24+
- Azure Service Bus namespace with:
  - A topic
  - A subscription under the topic
  - Shared Access Policy with `Send` and `Listen` rights

## Environment Variables

Set the following environment variables before running the app:

| Variable Name             | Description                                 |
|--------------------------|---------------------------------------------|
| `ASB_CONNECTION_STRING`  | Full connection string for Azure Service Bus <br> - *Optional*|
| `ASB_BROKER_URL`         | Azure Service Bus FQDN (e.g., `yournamespace.servicebus.windows.net`) <br> - *Required if the connection string is not provided* |
| `ASB_ACCESS_KEY_NAME`    | SAS Policy Name (e.g., `RootManageSharedAccessKey`) <br> - *Required if the connection string is not provided*|
| `ASB_ACCESS_KEY`         | SAS Policy Key <br> - *Required if the connection string is not provided*|
| `ASB_TOPIC`              | Topic name                                  |
| `ASB_SUBSCRIPTION`       | Subscription name under the topic           |

You can set them in your shell like this:

```bash
export ASB_BROKER_URL="your-servicebus.servicebus.windows.net"
export ASB_ACCESS_KEY_NAME="RootManageSharedAccessKey"
export ASB_ACCESS_KEY="your_access_key"
export ASB_TOPIC="your-topic-name"
export ASB_SUBSCRIPTION="your-subscription-name"
```

## Running the Application

```bash
go run main.go
```
- The server starts on http://localhost:8080
- The subscriber begins listening in the background

## Publishing a Message
Send a POST request to /publish with JSON body:
```bash
curl -X POST http://localhost:8080/publish \
     -H "Content-Type: application/json" \
     -d '{"message": "Hello from client!"}'
```
Expected Response:
```json
{
  "status": "Message published"
}
```
On the console, you'll see:
```
Published message: Hello from client!
Received message: Hello from client!
```
## Dependencies
- [go-amqp](github.com/Azure/go-amqp)
- [Gin](github.com/gin-gonic/gin)

Install dependencies with:
```bash
go mod tidy
```
