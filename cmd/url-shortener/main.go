package main

import (
	"fmt"
	handler "url-shornener/internal/handler"
	storage "url-shornener/internal/storage"
)

func main() {
	URL := "example.com"

	fmt.Println(URL)
	db, err := storage.Connect()
	//defer storage.Close(db)
	if err != nil {
		panic(err)
	}
	err = storage.CreateTable(db)
	if err != nil {
		panic(err)
	}
	storage.SetDB(db)

	fmt.Println("database connected")
	r := handler.NewRouter()

	api := r.Group("/api")
	{
		api.POST("/shorten", handler.ShortenURL)

	}
	r.GET("/:short", handler.RedirectURL)
	r.StaticFile("/", "./../../static/index.html") // замени путь при необходимости
	r.StaticFile("/index.html", "./../../static/index.html")
	r.Run(":8080")
}
