package main

import (
	"fmt"
	handler "url-shornener/internal/handler"
	storage "url-shornener/internal/storage"
)

func main() {

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
	r.StaticFile("/", "/home/boar/go-projects/url-shortener/static/index.html")
	r.StaticFile("/index.html", "/home/boar/go-projects/url-shortener/static/index.html")
	r.Run(":8080")
}
