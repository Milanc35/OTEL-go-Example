package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"

	"github.com/milan/OTEL-go-demo/logger"
	"github.com/segmentio/kafka-go"
	"go.elastic.co/apm/v2"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

const (
	NewOrderEvent = "NEW_ORDER"
)

// get random integer in range 1 to 8
func randomInt() int {
	min := 1
	max := 8
	return rand.Intn(max-min+1) + min
}

type Order struct {
	ID       string `json:"order_id,omitempty"`
	Quantity int    `json:"quantity,omitempty"`
	Name     string `json:"name,omitempty"`
}

// create list of orders with random item names
var orders = []Order{
	{ID: randomOrderId(), Quantity: 1, Name: "Item 1"},
	{ID: randomOrderId(), Quantity: 2, Name: "Item 2"},
	{ID: randomOrderId(), Quantity: 3, Name: "Item 3"},
	{ID: randomOrderId(), Quantity: 4, Name: "Item 4"},
	{ID: randomOrderId(), Quantity: 5, Name: "Item 5"},
	{ID: randomOrderId(), Quantity: 6, Name: "Item 6"},
	{ID: randomOrderId(), Quantity: 7, Name: "Item 7"},
	{ID: randomOrderId(), Quantity: 8, Name: "Item 8"},
	{ID: randomOrderId(), Quantity: 9, Name: "Item 9"},
}

func randomOrderId() string {
	// generate random string
	return "123456789"
}

func placeNewOrder(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), appName)
	defer span.End()

	logger := logger.LogHandle.WithContext(ctx)
	logger.Info("Placing new order...")
	// get req args
	errArg := r.URL.Query().Get("error")
	orderNumber := randomInt()
	order := orders[orderNumber]
	if errArg == "true" {
		err := order.processOrderWithError(ctx)
		if err != nil {
			logger.Error(fmt.Sprintf("Error processing order: %s", err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
	}
	err := order.processOrder(ctx)
	if err != nil {
		logger.Error(fmt.Sprintf("Error processing order: %v", err))
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - Something bad happened!"))
		return
	}
	err = order.processPayment(ctx)
	if err != nil {
		logger.Error(fmt.Sprintf("Error processing payment: %v", err))
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - Something bad happened!"))
		return
	}
	rawOrder, err := order.marshal()
	if err != nil {
		logger.Error(fmt.Sprintf("Error marshalling order: %v", err))
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - Something bad happened!"))
		return
	}
	kMsg := kafka.Message{
		Key:   []byte("NEW_ORDER"),
		Value: rawOrder,
	}
	writerK.WriteMessages(ctx, kMsg)
	w.WriteHeader(http.StatusOK)
	w.Write(rawOrder)
}

func (o Order) marshal() ([]byte, error) {
	return json.Marshal(o)
}

func (o Order) processOrder(ctx context.Context) error {
	fmt.Println("Processing order...")
	// create new apm span with attributes
	ctx, span := tracer.Start(ctx, "Order Procsing")
	span.SetAttributes(attribute.String("order.id", o.ID))
	span.SetAttributes(attribute.Int("order.quantity", o.Quantity))
	span.SetAttributes(attribute.String("order.name", o.Name))

	defer span.End()

	// simulate processing time
	fmt.Println("Order processed!")
	return nil
}

func (o Order) processOrderWithError(ctx context.Context) error {
	fmt.Println("Processing order...")
	// create new span with error
	ctx, span := tracer.Start(ctx, "processOrder")
	span.SetAttributes(attribute.String("order.id", o.ID))
	span.SetAttributes(attribute.Int("order.quantity", o.Quantity))
	span.SetAttributes(attribute.String("order.name", o.Name))
	span.End()
	o.ID = ""
	return o.processPayment(ctx)
}

func (o Order) processPayment(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "Process Payment")
	defer span.End()
	logger := logger.LogHandle.WithContext(ctx)
	logger.Info("Processing payment...")
	paymentSvcUrl := fmt.Sprintf("%s/payment", os.Getenv("PAYMENT_URL"))
	// wrap http with apmoprions

	client := &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
	rawOrder, err := o.marshal()
	req, _ := http.NewRequestWithContext(ctx, "GET", paymentSvcUrl, bytes.NewBuffer(rawOrder))
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil || res.StatusCode != http.StatusOK {
		apm.CaptureError(ctx, fmt.Errorf("error processing payment. response: %s %v", res.Status, err)).Send()
		err = fmt.Errorf("error processing payment. response: %s %v", res.Status, err)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		logger.Error(fmt.Sprintf("Error marshalling order: %v", err))
		return err
	}
	defer res.Body.Close()
	logger.Info("Payment service Response ", zap.String("response", string(body)))

	return err
}
