package webserver

import (
    "html/template"
    "net/http"
    //"strings"
    //"github.com/JohnDoe/Peerster/part3/message"
)

type Page struct {
    Name string
    Peers map[string]bool
    Messages map[string][]string
}

var IndexPage *Page
var GetChannel chan chan bool
var PostChannel chan [2]string

func index_handler(w http.ResponseWriter, r *http.Request) {
    //title := "Welcome to Peerster"
    //fmt.Fprintf(w, "<h1>%s</h1><div>%s</div>", page.Name,
                   //strings.Join(page.Peers, ","))
    //fmt.Fprintf(w, "Hi there, I love %s!", r.URL.Path[1:])
    request := make(chan bool)
    GetChannel<-request
    <-request
    t, _ := template.ParseFiles("webserver/index.html")
    t.Execute(w, IndexPage)
}

func handler(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    //fmt.Println(r.Form)
    //msg := message.ClientMessage{}
    for k,v := range r.Form {
        //fmt.Println("client_msg:",k, v)
        PostChannel<-[2]string{k, v[0]}
    }
}

func StartWebServer(get_channel chan chan bool, post_channel chan [2]string, index_page *Page) {
    IndexPage = index_page
    GetChannel = get_channel
    PostChannel = post_channel
    //fmt.Println("Starting Web Server")
    http.HandleFunc("/", index_handler)
    http.HandleFunc("/id", handler)
    http.HandleFunc("/message", handler)
    http.HandleFunc("/node", handler)
    http.ListenAndServe(":8080", nil)
}
