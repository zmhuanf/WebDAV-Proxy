package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	STORAGE_KEY            = "chatgpt-next-web"
	allowedWebDavEndpoints = []string{
		"https://dav.jianguoyun.com/dav/",
		"https://dav.dropdav.com/",
		"https://dav.box.com/dav",
		"https://nanao.teracloud.jp/dav/",
		"https://bora.teracloud.jp/dav/",
		"https://webdav.4shared.com/",
		"https://dav.idrivesync.com",
		"https://webdav.yandex.com",
		"https://app.koofr.net/dav/Koofr",
	}
)

func normalizeUrl(rawUrl string) *url.URL {
	u, err := url.Parse(rawUrl)
	if err != nil {
		log.Printf("%v, url = %s\n", err, rawUrl)
		return nil
	}
	return u
}

func isEndpointAllowed(endpoint string) bool {
	for _, allowedEndpoint := range allowedWebDavEndpoints {
		normalizedAllowedEndpoint := normalizeUrl(allowedEndpoint)
		normalizedEndpoint := normalizeUrl(endpoint)

		if normalizedEndpoint != nil && normalizedEndpoint.Hostname() == normalizedAllowedEndpoint.Hostname() && strings.HasPrefix(normalizedEndpoint.Path, normalizedAllowedEndpoint.Path) {
			return true
		}
	}
	return false
}

func proxyHandler(c *gin.Context) {
	if c.Request.Method == http.MethodOptions {
		c.JSON(http.StatusOK, gin.H{"body": "OK"})
		return
	}

	fileName := fmt.Sprintf("%s/backup.json", STORAGE_KEY)

	endpoint := c.Query("endpoint")
	proxyMethod := c.DefaultQuery("proxy_method", c.Request.Method)

	if !isEndpointAllowed(endpoint) {
		c.JSON(http.StatusBadRequest, gin.H{"error": true, "msg": "Invalid endpoint"})
		return
	}

	if !strings.HasSuffix(endpoint, "/") {
		endpoint += "/"
	}

	targetPath := fmt.Sprintf("%s%s", endpoint, fileName)

	if proxyMethod != "MKCOL" && proxyMethod != "GET" && proxyMethod != "PUT" {
		c.JSON(http.StatusForbidden, gin.H{"error": true, "msg": "you are not allowed to request " + targetPath})
		return
	}

	req, err := http.NewRequest(proxyMethod, targetPath, c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": true, "msg": "Failed to create request"})
		return
	}

	req.Header.Set("Authorization", c.Request.Header.Get("Authorization"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": true, "msg": "Failed to send request"})
		return
	}
	defer resp.Body.Close()

	fmt.Println("[Any Proxy]", targetPath, proxyMethod, resp.Status)

	for k, v := range resp.Header {
		for _, vv := range v {
			c.Header(k, vv)
		}
	}

	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	r := gin.Default()

	r.PUT("/api/webdav/chatgpt-next-web/*filepath", proxyHandler)
	r.GET("/api/webdav/chatgpt-next-web/*filepath", proxyHandler)
	r.OPTIONS("/api/webdav/chatgpt-next-web/*filepath", proxyHandler)

	exePath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	exeDir := filepath.Dir(exePath)
	outDir := filepath.Join(exeDir, "out")
	if _, err := os.Stat(outDir); os.IsNotExist(err) {
		log.Fatalf("路径不存在：%s\n", outDir)
	}

	r.GET("/", func(c *gin.Context) {
		c.File(filepath.Join(outDir, "index.html"))
	})
	r.NoRoute(func(c *gin.Context) {
		filePath := filepath.Join(outDir, c.Request.URL.Path)
		if _, err := os.Stat(filePath); err == nil {
			c.File(filePath)
		} else {
			c.JSON(404, gin.H{"message": "Not Found"})
		}
	})

	err = r.Run(":30000")
	if err != nil {
		log.Fatalln(err)
		return
	}
}
