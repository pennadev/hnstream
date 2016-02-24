package main

import (
	"net/http"
	"io/ioutil"
	"fmt"
	"encoding/json"
	"net"
	"log"
	"io"
	"sync"
	"time"
	"github.com/gorilla/websocket"
	"flag"
	"html/template"
)

type itemMap map[string]*NewsItem

var wsAddr = flag.String("addr", "localhost:8080", "http service address")

var homeTemplate = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<head>
<meta charset="utf-8">
<script>
window.addEventListener("load", function(evt) {
    var output = document.getElementById("output");
    var input = document.getElementById("input");
    var ws;
    var print = function(message) {
        var d = document.createElement("div");
        d.innerHTML = message;
        output.appendChild(d);
    };
    document.getElementById("open").onclick = function(evt) {
        if (ws) {
            return false;
        }
        ws = new WebSocket("{{.}}");
        ws.onopen = function(evt) {
            print("OPEN");
        }
        ws.onclose = function(evt) {
            print("CLOSE");
            ws = null;
        }
        ws.onmessage = function(evt) {
            print("RESPONSE: " + evt.data);
        }
        ws.onerror = function(evt) {
            print("ERROR: " + evt.data);
        }
        return false;
    };
    document.getElementById("send").onclick = function(evt) {
        if (!ws) {
            return false;
        }
        print("SEND: " + input.value);
        ws.send(input.value);
        return false;
    };
    document.getElementById("close").onclick = function(evt) {
        if (!ws) {
            return false;
        }
        ws.close();
        return false;
    };
});
</script>
</head>
<body>
<table>
<tr><td valign="top" width="50%">
<p>Click "Open" to create a connection to the server,
"Send" to send a message to the server and "Close" to close the connection.
You can change the message and send multiple times.
<p>
<form>
<button id="open">Open</button>
<button id="close">Close</button>
<p><input id="input" type="text" value="Hello world!">
<button id="send">Send</button>
</form>
</td><td valign="top" width="50%">
<div id="output"></div>
</td></tr></table>
</body>
</html>
`))

type NewsItem struct {
	Tags        []string `json:"_tags"`
	Author      string `json:"author"`
	CreatedAt   time.Time `json:"created_at"`
	CreatedAtI  int `json:"created_at_i"`
	NumComments int `json:"num_comments"`
	ObjectID    string `json:"objectID"`
	Points      int `json:"points"`
	Title       string `json:"title"`
	URL         string `json:"url"`
}

func request() []interface{} {
	res, err := http.Get("http://hn.algolia.com/api/v1/search_by_date?tags=story")
	if err == nil {
		body, _ := ioutil.ReadAll(res.Body)
		var i interface{}
		json.Unmarshal(body, &i)
		m := i.(map[string]interface{})
		return m["hits"].([]interface{})
	} else {
		return nil
	}
}

func mapItems(iMap itemMap, data []interface{}) {
	for _, item := range data {
		j, _ := json.MarshalIndent(item, "", "    ")
		newsItem := new(NewsItem)
		json.Unmarshal(j, &newsItem)
		if iMap[newsItem.ObjectID] == nil {
			iMap[newsItem.ObjectID] = newsItem;
			//fmt.Println(iMap[newsItem.ObjectID])
			fmt.Println("%s",string(j))
		}
	}
}

const listenAddr = "0.0.0.0:4000"

func tcpEcho() {
	l, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatal(err)
	}
	for {
		c, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go io.Copy(c, c)
	}
}

func poll() {
	iMap := make(itemMap)
	for {
		hits := request()
		mapItems(iMap, hits)
		time.Sleep(5 * time.Second)
	}
}

var upgrader = websocket.Upgrader{} // use default options

func wsEcho(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		log.Printf("recv: %s", message)
		err = c.WriteMessage(mt, message)
		if err != nil {
			log.Println("write:", err)
			break
		}
	}
}

func home(w http.ResponseWriter, r *http.Request) {
	homeTemplate.Execute(w, "ws://" + r.Host + "/echo")
}

func main() {
	wg := sync.WaitGroup{}
	wg.Add(3)
	go poll()
	go tcpEcho()

	go func() {
		flag.Parse()
		log.SetFlags(0)
		http.HandleFunc("/echo", wsEcho)
		http.HandleFunc("/", home)
		log.Fatal(http.ListenAndServe(*wsAddr, nil))
	}()

	wg.Wait()
}


