package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"runtime"

	"github.com/gocolly/colly"
)

//Store is an abstraction of the different vendors
type Store struct {
	Name      string
	URL       string
	Selector  string
	Attribute string
	Title     string
	Value     string
}

//Prospect returns the result and the response code of a product availability query after scrape
type Prospect struct {
	Result     string
	StatusCode string
}

var (
	amazon = Store{
		Name:      "Amazon",
		URL:       "https://www.amazon.es/Playstation-Consola-PlayStation-5/dp/B08KKJ37F7/ref=sr_1_2",
		Selector:  "input",
		Attribute: "id",
		Value:     "add-to-cart-button",		
		Title:	   "Consola PlayStation 5: Amazon.es: Videojuegos",
	}
	game = Store{
		Name:      "Game",
		URL:       "https://www.game.es/HARDWARE/CONSOLA/PLAYSTATION-5/CONSOLA-PLAYSTATION-5/183224",
		Selector:  "button",
		Attribute: "title",
		Value:     "comprar",
	}
	eci = Store{
		Name:      "ECI",
		URL:       "https://www.elcorteingles.es/videojuegos/A37046604/",
		Selector:  "button",
		Attribute: "data-synth",
		Value:     "LOCATOR_ADD_CART_BUTTON",
	}
	mm = Store{
		Name:      "MM",
		URL:       "https://www.mediamarkt.es/es/product/_consola-sony-ps5-825-gb-4k-hdr-blanco-1487016.html",
		Selector:  "a[href]",
		Attribute: "id",
		Value:     "pdp-add-to-cart",
	}
	pcc = Store{
		Name:      "PCComponentes",
		URL:       "https://www.pccomponentes.com/sony-playstation-5",
		Selector:  "button",
		Attribute: "class",
		Value:     "buy-button",
	}

	scrappedURLs []string = []string{amazon.URL, pcc.URL, game.URL, eci.URL, mm.URL}
	firefox string = "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:84.0) Gecko/20100101 Firefox/84.0"
	adminID string = os.Getenv("ADMIN_TELEGRAM_ID")
	groupID string = os.Getenv("GROUP_TELEGRAM_ID")
	token   string = os.Getenv("TELEGRAM_BOT_TOKEN")
	fire    string =  "\U0001F525"	
	m runtime.MemStats

)

func main() {	

	if adminID == "" {
		log.Fatal("FATAL! ADMIN_TELEGRAM_ID not present...")
	}
	if groupID == "" {
		log.Fatal("FATAL! GROUP_TELEGRAM_ID not present...")
	}
	if token == "" {
		log.Fatal("FATAL! TELEGRAM_BOT_TOKEN not present...")
	}
	
	var wg sync.WaitGroup
	sendTelegramMsg(adminID, "Bot restarted. Keep going!")				
			
	wg.Add(1)
	go func() {
		for true {
			c := colly.NewCollector()
			c.UserAgent = firefox
			checkStock(c, amazon)
		}

	}()

	go func() {
		for true {
			c := colly.NewCollector()
			c.UserAgent = firefox
			checkStock(c, game)
		}
	}()

	go func() {
		for true {
			c := colly.NewCollector()
			c.UserAgent = firefox
			checkStock(c, eci)
		}
	}()

	go func() {
		for true {
			c := colly.NewCollector()
			c.UserAgent = firefox
			checkStock(c, mm)
		}
	}()

	go func() {
		for true {
			c := colly.NewCollector()
			c.UserAgent = firefox
			checkStock(c, pcc)
		}
	}()

	go func() {
		for range time.Tick(24 * time.Hour) {
			sendTelegramMsg(groupID, fmt.Sprintf("We keep waiting stock for:\n%s%v", fire, strings.Join(scrappedURLs, fmt.Sprintf("\n%s", fire))))
		}	
	}()

	go func(){
		for range time.Tick(5 * time.Minute) {
			sendTelegramMsg(adminID, printMemUsage())
		}
	}()

	wg.Wait()	

}

func checkStock(c *colly.Collector, store Store) (p Prospect) {
	var valid bool = false	
	p.Result = fmt.Sprintf("No stock in %s,", store.Name)	
	c.OnHTML(store.Selector, func(e *colly.HTMLElement) {
		input := e.Attr(store.Attribute)
		p.StatusCode = strconv.Itoa(e.Response.StatusCode)
		if strings.Contains(strings.ToLower(input), store.Value) {
			p.Result = fmt.Sprintf("You can buy it! %s\n", e.Request.URL)
			valid = true								
		}
	})
	c.OnHTML("head title", func(e *colly.HTMLElement) { // Title
        if store.Name=="Amazon" && store.Title != e.Text{
			valid = false
			fmt.Println("Take care: Amazon doing thing with product page!")
		}
    })


	c.Visit(store.URL)
	c.Wait()
		
	if valid{
		notify(p)
	}
	time.Sleep(time.Duration(banControl(p)) * time.Second)

	return
}

func notify(p Prospect) {
	r, err := sendTelegramMsg(groupID, p.Result)
	if err != nil {
		log.Printf("Error sending msg: %s", err)
	}
	fmt.Println(r)
}

func banControl(p Prospect) (interval int) {
	if p.StatusCode == "200" {
		interval = (rand.Intn(15) + 5)
	} else {
		log.Printf("ERROR! - Status code is not 200... status code: %s delaying next scrap  x100\n", p.StatusCode)
		l, err := sendTelegramMsg(adminID, fmt.Sprintf("Problems with an url %s: Status code is %s", p.Result, p.StatusCode))
		if err != nil {
			log.Printf("ERROR! - %s", err)
		}
		log.Printf("Error reported to administrator. %s", l)
		interval = 3600
	}
	return
}

func sendTelegramMsg(chatID string, text string) (string, error) {
	log.Printf("Sending %s to chat_id: %s", text, chatID)
	response, err := http.PostForm(
		fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token),
		url.Values{
			"chat_id": {chatID},
			"text":    {text},
		})

	if err != nil {
		log.Printf("error when posting text to the chat: %s", err.Error())
		return "", err
	}
	defer response.Body.Close()

	var bodyBytes, errRead = ioutil.ReadAll(response.Body)
	if errRead != nil {
		log.Printf("error in parsing telegram answer %s", errRead.Error())
		return "", err
	}
	bodyString := string(bodyBytes)
	log.Printf("Body of Telegram Response: %s", bodyString)

	return bodyString, nil
}

func printMemUsage() string {
	
	runtime.ReadMemStats(&m)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	return fmt.Sprintf("Bot instance stats:\n\tAlloc = %v MiB\n\tHeapAlloc = %v MiB\n\tSys = %v MiB\n\tNumGC = %v\n", bToMb(m.Alloc), bToMb(m.HeapAlloc), bToMb(m.Sys), m.NumGC)

}

func bToMb(b uint64) uint64 {
    return b / 1024 / 1024
}
