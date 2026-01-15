package main

import (
	"crypto/md5"
	"embed"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	VERSION = "1.8.79"
)

var (
	ConfigSource string
	WebMode      string
	WebPort      int
	WebEnable    bool
	WebUsername  string
	WebPassword  string
	ApiKey       string
)

var (
	vkBakDict = make(map[string]int64)
	tokens    = make(map[string]string) // token -> username
)

//go:embed index.html
var indexHTML embed.FS

func MD5(str string) string {
	data := []byte(str)
	has := md5.Sum(data)
	md5str := fmt.Sprintf("%x", has)
	return md5str
}

func getEnvString(name string, value string) string {
	ret := os.Getenv(name)
	if ret == "" {
		return value
	} else {
		return ret
	}
}

func getEnvInt(name string, value int) int {
	env := os.Getenv(name)
	if ret, err := strconv.Atoi(env); env == "" || err != nil {
		return value
	} else {
		return ret
	}
}

func init() {
	flag.StringVar(&ConfigSource, "c", "default", "config source default or env.")
	flag.StringVar(&WebMode, "mode", gin.ReleaseMode, "wol web mode: debug, release, test.")
	flag.IntVar(&WebPort, "port", 9090, "wol web port: 0-65535")
	flag.BoolVar(&WebEnable, "web", true, "wol web page switch: true or false.")
	flag.StringVar(&WebUsername, "username", "", "wol web page login username.")
	flag.StringVar(&WebPassword, "password", "", "wol web page login password.")
	flag.StringVar(&ApiKey, "key", "false", "wol web api key, key length greater than 6.")
}

func main() {
	flag.Parse()

	fmt.Printf("Start Run WolGoWeb...\n\n")
	fmt.Printf("Version: %s\n\n", VERSION)

	names, err := NetworkInterfaceNames()
	if err == nil {
		fmt.Printf("Network Interface Names: %+q\n\n", names)
	}

	if ConfigSource == "env" {
		WebMode = getEnvString("MODE", WebMode)
		WebPort = getEnvInt("PORT", WebPort)
		WebEnable = getEnvString("WEB", strconv.FormatBool(WebEnable)) == "true"
		WebUsername = getEnvString("USERNAME", WebUsername)
		WebPassword = getEnvString("PASSWORD", WebPassword)
		ApiKey = getEnvString("KEY", ApiKey)
	}

	// Initialize random seed
	rand.Seed(time.Now().UnixNano())

	// Load devices
	if err := LoadDevices(); err != nil {
		fmt.Printf("Warning: Failed to load devices: %v\n", err)
	}

	gin.SetMode(WebMode)

	r := gin.Default()

	// 添加 Web 服务
	if WebEnable {
		// 忽略 Chrome DevTools 的噪声请求
		r.GET("/.well-known/appspecific/com.chrome.devtools.json", func(c *gin.Context) {
			c.Status(204)
		})

		r.GET("/", GetIndex)
		r.GET("/index", GetIndex)

		r.POST("/auth/login", Login)

		api := r.Group("/api")
		api.Use(AuthMiddleware())
		{
			api.GET("/devices", GetDevices)
			api.POST("/devices", CreateDevice)
			api.DELETE("/devices/:id", DeleteDevice)
			api.POST("/wake/:id", WakeDevice)
			api.GET("/scan", GetScan)
		}
	}
	// Old WOL API
	r.GET("/wol", GetWol)

	fmt.Printf("WolGoWeb Runing [port:%d, key:%s, web:%s]\n", WebPort, ApiKey, strconv.FormatBool(WebEnable))

	err = r.Run(fmt.Sprintf(":%d", WebPort))
	if err != nil {
		fmt.Println(err.Error())
	}
}

func GenerateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func Login(c *gin.Context) {
	var login struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&login); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// Basic Auth Logic based on flags
	// If WebUsername/Password are empty, login is permissive or disabled?
	// Requirement says "If not logged in, force login".
	// So we assume username/password MUST be set for this feature to work well.
	if (WebUsername == "" && WebPassword == "") || (login.Username == WebUsername && login.Password == WebPassword) {
		token := GenerateToken()
		tokens[token] = login.Username
		c.JSON(http.StatusOK, gin.H{"token": token})
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
	}
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if WebUsername == "" && WebPassword == "" {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if _, ok := tokens[token]; ok {
			c.Next()
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		}
	}
}

func GetDevices(c *gin.Context) {
	if c.Query("refresh") == "true" {
		UpdateStatuses()
	}
	c.JSON(http.StatusOK, GetAllDevices())
}

func GetScan(c *gin.Context) {
	devices, err := ScanLocalNetwork()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, devices)
}

func CreateDevice(c *gin.Context) {
	var d Device
	if err := c.ShouldBindJSON(&d); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate MAC
	if _, err := New(d.MAC); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid MAC address"})
		return
	}

	d.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	if err := AddDevice(d); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, d)
}

func DeleteDevice(c *gin.Context) {
	id := c.Param("id")
	if err := RemoveDevice(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Device deleted"})
}

func WakeDevice(c *gin.Context) {
	id := c.Param("id")
	dev, err := GetDevice(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Device not found"})
		return
	}

	if err := Wake(dev.MAC, dev.IP, dev.Port, ""); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	} else {
		c.JSON(http.StatusOK, gin.H{"message": "Wake signal sent"})
	}
}

func VerifyAuth(key string, mac string, vk int64, token string) (int, string) {
	err := 0
	message := "OK"
	if len(key) >= 6 {
		timeUnix := time.Now().Unix()
		fmt.Printf("now=%d, vk=%d\n", timeUnix, vk)
		if len(token) != 32 {
			err = 101
			message = "No authority."
		} else if timeUnix-vk > 30 || vk-timeUnix > 1 {
			err = 102
			message = "The value of Time is no longer in the valid range."
		} else if bakVK, ok := vkBakDict[mac]; ok && bakVK == vk {
			err = 103
			message = "Time value repetition."
		} else if MD5(ApiKey+mac+fmt.Sprintf("%d", vk)) != token {
			err = 104
			message = "No authority token."
		} else {
			vkBakDict[mac] = vk
		}
	}
	return err, message
}

func GetIndex(c *gin.Context) {
	data, err := indexHTML.ReadFile("index.html")
	if err != nil {
		c.String(500, "Failed to load page")
		return
	}
	html := strings.ReplaceAll(string(data), "<!--VERSION-->", VERSION)
	c.Data(200, "text/html; charset=utf-8", []byte(html))
}

func GetWol(c *gin.Context) {
	mac := c.Query("mac")
	ip := c.DefaultQuery("ip", "255.255.255.255")
	port := c.DefaultQuery("port", "9")
	network := c.DefaultQuery("network", "")
	token := c.DefaultQuery("token", "")
	vk, _ := strconv.ParseInt(c.DefaultQuery("time", "0"), 10, 64)
	if errAuth, messageAuth := VerifyAuth(ApiKey, mac, vk, token); errAuth == 0 {
		err := Wake(mac, ip, port, network)
		if err != nil {
			c.JSON(200, gin.H{
				"error":   100,
				"message": fmt.Sprintf("%s", err),
			})
		} else {
			c.JSON(200, gin.H{
				"error":   0,
				"message": fmt.Sprintf("Wake Success Mac:%s", mac),
			})
		}
	} else {
		c.JSON(200, gin.H{
			"error":   errAuth,
			"message": messageAuth,
		})
	}
}
