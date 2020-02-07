package main

import (
	"net/http"
	"webhooks/common/app"
)

var App *app.App

func init() {
	App = app.AppInitStrict(app.StorageTypeDynamoDb)
}

func main() {
	http.HandleFunc("/webhook", App.CreateWebHookHttpHandler())
}
