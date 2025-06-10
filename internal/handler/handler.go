package handler

import (
	"fmt"
	"net/http"

	db "url-shornener/internal/storage"
	url_short "url-shornener/pkg/urlshortener"

	"github.com/gin-gonic/gin"
)

func NewRouter() *gin.Engine {
	return gin.Default()
}
func ShortenURL(c *gin.Context) {
	// Структура для получения JSON
	var request struct {
		URL string `json:"url" binding:"required"`
	}

	// Парсим JSON из тела запроса
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	fmt.Println("Received URL:", request.URL)

	shortURL, err := url_short.URLtoShortURL(request.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	db.AddURL(request.URL, shortURL)

	c.JSON(http.StatusOK, gin.H{
		"message":   "URL shortened successfully",
		"short_url": shortURL,
	})
}
func RedirectURL(c *gin.Context) {
	shortURL := c.Param("short")
	if shortURL == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Short URL is required",
		})
		return
	}

	originalURL, err := db.GetOriginalURL(shortURL)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	if originalURL == "" {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": "Short URL not found",
		})
		return
	}

	c.Redirect(http.StatusMovedPermanently, originalURL)
}
